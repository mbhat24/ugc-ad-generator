import { mkdtemp, rm, writeFile } from "fs/promises";
import { tmpdir } from "os";
import { join } from "path";
import { query, type Ad, type Product } from "./db";
import { log } from "./log";
import { burnCaptions } from "./captions";
import { privateMedia, storageEnabled, uploadVideo } from "./storage";
import { provider, RetryLater } from "./video";
import { enqueueAd } from "./queue";

const MAX_AGE_MS = 25 * 60 * 1000;
const POLL_DELAY_S = 8;

type Row = { id: string; hook: string; script: string; caption: string; hashtags: string[]; avatar: string; status: Ad["status"]; video_job_id: string | null; created_at: Date; user_id: string; product: Product };

export async function fail(adId: string, userId: string, error: string) {
  const r = await query("update ads set status = 'failed', error = $2, updated_at = now() where id = $1 and status in ('pending','processing') returning id", [adId, error.slice(0, 300)]);
  if (r.length) log.warn("ad failed", { adId, error });
  if (r.length) await query("insert into credit_ledger (user_id, delta, reason, ref) values ($1, 1, 'refund', $2) on conflict do nothing", [userId, adId]);
}

async function finalize(row: Row, sourceUrl: string) {
  let url: string | null = sourceUrl;
  let key: string | null = null;
  let stored = false;
  if (storageEnabled) {
    const dir = await mkdtemp(join(tmpdir(), "ad-"));
    try {
      const res = await fetch(sourceUrl, { signal: AbortSignal.timeout(120000) });
      if (!res.ok) throw new Error(`download ${res.status}`);
      await writeFile(join(dir, "in.mp4"), Buffer.from(await res.arrayBuffer()));
      let file = join(dir, "in.mp4");
      try { await burnCaptions(dir, "in.mp4", "out.mp4", `${row.hook}. ${row.script}`); file = join(dir, "out.mp4"); }
      catch (e) { log.warn("captions skipped", { adId: row.id, err: e }); }
      const up = await uploadVideo(file, `videos/${row.user_id}/${row.id}.mp4`);
      stored = true;
      if (privateMedia) { key = up; url = null; } else url = up;
    } catch (e) {
      log.warn("storage fallback", { adId: row.id, err: e });
    } finally {
      await rm(dir, { recursive: true, force: true });
    }
  }
  await query("update ads set status = 'done', video_url = $2, video_key = $3, error = null, updated_at = now() where id = $1 and status = 'processing'", [row.id, url, key]);
  if (stored && row.video_job_id) await provider.cleanup?.(row.video_job_id);
}

export async function processAd(adId: string) {
  const row = (await query<Row>("select a.*, p.user_id, p.product from ads a join projects p on p.id = a.project_id where a.id = $1", [adId]))[0];
  if (!row || (row.status !== "pending" && row.status !== "processing")) return;
  try {
    if (Date.now() - row.created_at.getTime() > MAX_AGE_MS) return await fail(adId, row.user_id, "Video generation timed out");
    if (!row.video_job_id) {
      const jobId = await provider.start({ ...rowToAd(row) }, row.product);
      await query("update ads set status = 'processing', video_job_id = $2, updated_at = now() where id = $1", [adId, jobId]);
      return await enqueueAd(adId, POLL_DELAY_S);
    }
    const s = await provider.poll(row.video_job_id);
    if (s.status === "processing") { await query("update ads set updated_at = now() where id = $1", [adId]); return await enqueueAd(adId, POLL_DELAY_S); }
    if (s.status === "failed" || !s.videoUrl) return await fail(adId, row.user_id, s.error ?? "Generation failed");
    await finalize(row, s.videoUrl);
  } catch (e) {
    if (e instanceof RetryLater) { await query("update ads set updated_at = now() where id = $1", [adId]); return await enqueueAd(adId, e.seconds); }
    await fail(adId, row.user_id, (e as Error).message);
  }
}

const rowToAd = (r: Row): Ad => ({ id: r.id, hook: r.hook, script: r.script, caption: r.caption, hashtags: r.hashtags, avatar: r.avatar, status: r.status });

export async function failFromDeadLetter(adId: string) {
  const row = (await query<{ user_id: string }>("select p.user_id from ads a join projects p on p.id = a.project_id where a.id = $1", [adId]))[0];
  if (row) await fail(adId, row.user_id, "Video generation failed after retries");
}

export async function reconcile() {
  const stale = await query<{ id: string }>("select id from ads where status in ('pending','processing') and updated_at < now() - interval '3 minutes' order by updated_at limit 500");
  for (const a of stale) await enqueueAd(a.id, 0, true);
  const cleaned = await query("delete from rate_limits where window_start < now() - interval '2 hours' returning 1");
  if (stale.length) log.warn("reconciler re-enqueued stale ads", { count: stale.length });
  return { requeued: stale.length, cleaned: cleaned.length };
}
