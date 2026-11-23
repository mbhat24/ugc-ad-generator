import { createHmac, timingSafeEqual } from "crypto";
import { query } from "@/lib/db";
import { enqueueAd } from "@/lib/queue";

function verify(id: string, ts: string, body: string, header: string) {
  const secret = process.env.HIGGSFIELD_WEBHOOK_SECRET;
  if (!secret || !id || !ts || Math.abs(Date.now() / 1000 - Number(ts)) > 300) return false;
  const key = Buffer.from(secret.replace(/^whsec_/, ""), "base64");
  const expected = createHmac("sha256", key).update(`${id}.${ts}.${body}`).digest();
  return header.split(" ").some((part) => {
    const [v, sig] = part.split(",");
    const got = Buffer.from(sig ?? "", "base64");
    return v === "v1" && got.length === expected.length && timingSafeEqual(got, expected);
  });
}

export async function POST(req: Request) {
  const body = await req.text();
  if (!verify(req.headers.get("webhook-id") ?? "", req.headers.get("webhook-timestamp") ?? "", body, req.headers.get("webhook-signature") ?? "")) return new Response("Bad signature", { status: 400 });
  let requestId: unknown;
  try { requestId = JSON.parse(body).request_id; } catch { return new Response("Bad body", { status: 400 }); }
  if (typeof requestId !== "string") return new Response("Bad body", { status: 400 });
  const ads = await query<{ id: string }>("select id from ads where video_job_id = $1 and status = 'processing'", [requestId]);
  await Promise.all(ads.map((a) => enqueueAd(a.id)));
  return Response.json({ received: true });
}
