import { mkdtemp, rm, writeFile } from "fs/promises";
import { tmpdir } from "os";
import { join } from "path";
import { query, type Ad, type Product } from "./db";
import { burnCaptions } from "./captions";
import { storageEnabled, uploadVideo } from "./storage";
import { provider } from "./video";
import { enqueueAd } from "./queue";

const MAX_AGE_MS = 25 * 60 * 1000;
const POLL_DELAY_S = 8;

type Row = { id: string; hook: string; script: string; caption: string; hashtags: string[]; avatar: string; status: Ad["status"]; video_job_id: string | null; created_at: Date; user_id: string; product: Product };

async function fail(adId: string, userId: string, error: string) {
  const r = await query("update ads set status = 'failed', error = $2 where id = $1 and status in ('pending','processing') returning id", [adId, error.slice(0, 300)]);
  if (r.length) await query("insert into credit_ledger (user_id, delta, reason, ref) values ($1, 1, 'refund', $2) on conflict do nothing", [userId, adId]);
}

async function finalize(row: Row, sourceUrl: string) {
  let url = sourceUrl;
  if (storageEnabled) {
    const dir = await mkdtemp(join(tmpdir(), "ad-"));
    try {
      const res = await fetch(sourceUrl, { signal: AbortSignal.timeout(120000) });
      if (!res.ok) throw new Error(`download ${res.status}`);
      await writeFile(join(dir, "in.mp4"), Buffer.from(await res.arrayBuffer()));
      let file = join(dir, "in.mp4");
      try { await burnCaptions(dir, "in.mp4", "out.mp4", `${row.hook}. ${row.script}`); file = join(dir, "out.mp4"); }
      catch (e) { console.warn(`captions skipped for ${row.id}:`, (e as Error).message); }
      url = await uploadVideo(file, `videos/${row.user_id}/${row.id}.mp4`);
    } catch (e) {
      console.warn(`storage fallback for ${row.id}:`, (e as Error).message);
    } finally {
      await rm(dir, { recursive: true, force: true });
    }
  }
  await query("update ads set status = 'done', video_url = $2, error = null where id = $1 and status = 'processing'", [row.id, url]);
}

export async function processAd(adId: string) {
  const row = (await query<Row>("select a.*, p.user_id, p.product from ads a join projects p on p.id = a.project_id where a.id = $1", [adId]))[0];
  if (!row || (row.status !== "pending" && row.status !== "processing")) return;
  try {
    if (Date.now() - row.created_at.getTime() > MAX_AGE_MS) return await fail(adId, row.user_id, "Video generation timed out");
    if (!row.video_job_id) {
      const jobId = await provider.start({ ...rowToAd(row) }, row.product);
      await query("update ads set status = 'processing', video_job_id = $2 where id = $1", [adId, jobId]);
      return await enqueueAd(adId, POLL_DELAY_S);
    }
    const s = await provider.poll(row.video_job_id);
    if (s.status === "processing") return await enqueueAd(adId, POLL_DELAY_S);
    if (s.status === "failed" || !s.videoUrl) return await fail(adId, row.user_id, s.error ?? "Generation failed");
    await finalize(row, s.videoUrl);
  } catch (e) {
    await fail(adId, row.user_id, (e as Error).message);
  }
}

const rowToAd = (r: Row): Ad => ({ id: r.id, hook: r.hook, script: r.script, caption: r.caption, hashtags: r.hashtags, avatar: r.avatar, status: r.status });
