import type { Ad, Project } from "../db";

export type JobState = { status: "processing" | "done" | "failed"; videoUrl?: string; error?: string };
export class RetryLater extends Error {
  constructor(public seconds: number, message = "Provider busy") { super(message); }
}
export interface VideoProvider {
  start(ad: Ad, product: Project["product"]): Promise<string>;
  poll(jobId: string): Promise<JobState>;
  cleanup?(jobId: string): Promise<void>;
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
const headers = () => ({ authorization: `Key ${process.env.HIGGSFIELD_KEY_ID}:${process.env.HIGGSFIELD_KEY_SECRET}`, "content-type": "application/json" });
const webhookUrl = () => process.env.APP_URL?.startsWith("https://") && process.env.HIGGSFIELD_WEBHOOK_SECRET ? `${process.env.APP_URL.replace(/\/$/, "")}/api/higgsfield/webhook` : null;

const higgsfield: VideoProvider = {
  async start(ad, product) {
    const prompt = `Vertical 9:16 UGC-style selfie video. ${ad.avatar}. Holding and showing "${product.title}" to the camera, speaking naturally: "${ad.hook}. ${ad.script}"`;
    const body: Record<string, unknown> = { prompt, aspect_ratio: "9:16", duration: 15 };
    if (product.image) body.image_url = product.image;
    const hook = webhookUrl();
    const url = `${HF}/${process.env.HIGGSFIELD_MODEL}${hook ? `?hf_webhook=${encodeURIComponent(hook)}` : ""}`;
    const r = await fetch(url, { method: "POST", headers: { ...headers(), "idempotency-key": ad.id }, body: JSON.stringify(body), signal: AbortSignal.timeout(30000) });
    if (r.status === 429 || r.status === 503) throw new RetryLater(Number(r.headers.get("retry-after")) || 30);
    if (!r.ok) throw new Error(`Higgsfield error ${r.status}: ${(await r.text()).slice(0, 200)}`);
    return (await r.json()).request_id as string;
  },
  async poll(requestId) {
    const r = await fetch(`${HF}/requests/${requestId}/status`, { headers: headers(), signal: AbortSignal.timeout(20000) });
    if (r.status === 404) return { status: "failed", error: "Generation request not found" };
    if (!r.ok) return { status: "processing" };
    const j = await r.json();
    if (j.status === "completed") return j.video?.url ? { status: "done", videoUrl: j.video.url } : { status: "failed", error: "No video returned" };
    if (j.status === "nsfw") return { status: "failed", error: "Rejected by content moderation" };
    if (["failed", "canceled"].includes(j.status)) return { status: "failed", error: j.error ?? `Generation ${j.status}` };
    return { status: "processing" };
  },
  async cleanup(requestId) {
    await fetch(`${HF}/requests/${requestId}`, { method: "DELETE", headers: headers(), signal: AbortSignal.timeout(20000) }).catch(() => {});
  },
};

export const provider: VideoProvider = process.env.HIGGSFIELD_KEY_ID && process.env.HIGGSFIELD_KEY_SECRET && process.env.HIGGSFIELD_MODEL ? higgsfield : mock;
export const isMock = provider === mock;
