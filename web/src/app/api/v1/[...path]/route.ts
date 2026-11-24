import { API_URL } from "@/lib/api";
import { accessToken } from "@/lib/session";
import { sameOrigin } from "@/lib/origin";

const PASS_REQUEST = ["content-type", "stripe-signature", "webhook-id", "webhook-timestamp", "webhook-signature", "x-forwarded-for", "user-agent"];
const PASS_RESPONSE = ["content-type", "content-disposition", "retry-after"];

async function forward(req: Request, ctx: RouteContext<"/api/v1/[...path]">) {
  const { path } = await ctx.params;
  const isWebhook = path[0] === "webhooks";
  const mutating = !["GET", "HEAD"].includes(req.method);
  if (mutating && !isWebhook && !sameOrigin(req)) return Response.json({ error: "Bad origin", code: "bad_origin" }, { status: 403 });

  const headers = new Headers();
  for (const h of PASS_REQUEST) { const v = req.headers.get(h); if (v) headers.set(h, v); }
  if (!isWebhook) {
    const token = await accessToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }
  const url = new URL(req.url);
  const res = await fetch(`${API_URL}/v1/${path.map(encodeURIComponent).join("/")}${url.search}`, {
    method: req.method, headers, body: mutating ? await req.arrayBuffer() : undefined, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(110_000),
  }).catch(() => null);
  if (!res) return Response.json({ error: "Service unavailable. Please retry.", code: "unavailable" }, { status: 502 });

  const out = new Headers({ "cache-control": "no-store" });
  for (const h of PASS_RESPONSE) { const v = res.headers.get(h); if (v) out.set(h, v); }
  return new Response(res.body, { status: res.status, headers: out });
}

export { forward as GET, forward as POST, forward as DELETE };
