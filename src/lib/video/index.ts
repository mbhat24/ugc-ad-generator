import type { Ad, Project } from "../db";

export type JobState = { status: "processing" | "done" | "failed"; videoUrl?: string; error?: string };
export interface VideoProvider {
  start(ad: Ad, product: Project["product"]): Promise<string>;
  poll(jobId: string): Promise<JobState>;
}

const MOCK_VIDEO = "https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4";

const mock: VideoProvider = {
  async start() { return `mock_${Date.now()}`; },
  async poll(jobId) {
    const started = Number(jobId.split("_")[1]);
    return Date.now() - started > 6000 ? { status: "done", videoUrl: MOCK_VIDEO } : { status: "processing" };
  },
};

const HF = "https://api.higgsfield.ai";
const auth = () => ({ authorization: `Key ${process.env.HIGGSFIELD_KEY_ID}:${process.env.HIGGSFIELD_KEY_SECRET}`, "content-type": "application/json" });

const higgsfield: VideoProvider = {
  async start(ad, product) {
    const prompt = `Vertical 9:16 UGC-style selfie video. ${ad.avatar}. Holding and showing "${product.title}" to the camera, speaking naturally: "${ad.hook}. ${ad.script}"`;
    const body: Record<string, unknown> = { prompt, aspect_ratio: "9:16", duration: 15 };
    if (product.image) body.image_url = product.image;
    const r = await fetch(`${HF}/${process.env.HIGGSFIELD_MODEL}`, { method: "POST", headers: auth(), body: JSON.stringify(body) });
    if (!r.ok) throw new Error(`Higgsfield error ${r.status}: ${(await r.text()).slice(0, 200)}`);
    const j = await r.json();
    return j.status_url ?? `${HF}/requests/${j.request_id}/status`;
  },
  async poll(statusUrl) {
    const r = await fetch(statusUrl, { headers: auth() });
    if (!r.ok) return { status: "processing" };
    const j = await r.json();
    if (j.status === "completed") return { status: "done", videoUrl: j.video?.url };
    if (["failed", "nsfw", "canceled"].includes(j.status)) return { status: "failed", error: j.error ?? `Generation ${j.status}` };
    return { status: "processing" };
  },
};

export const provider: VideoProvider = process.env.HIGGSFIELD_KEY_ID && process.env.HIGGSFIELD_KEY_SECRET && process.env.HIGGSFIELD_MODEL ? higgsfield : mock;
export const isMock = provider === mock;
