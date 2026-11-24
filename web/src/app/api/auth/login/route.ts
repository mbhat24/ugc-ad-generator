import { cookies } from "next/headers";
import { API_URL } from "@/lib/api";
import { DEV_COOKIE, devAuth } from "@/lib/devauth";
import { supabaseServer } from "@/lib/supabase";
import { sameOrigin } from "@/lib/origin";

const bad = (error: string, status: number) => Response.json({ error }, { status });

export async function POST(req: Request) {
  if (!sameOrigin(req)) return bad("Bad origin", 403);
  const body = await req.json().catch(() => null);
  const email = String(body?.email ?? "").trim().toLowerCase();
  const password = String(body?.password ?? "");
  if (!email || email.length > 254) return bad("Enter a valid email", 400);

  if (devAuth) {
    const r = await fetch(`${API_URL}/v1/dev/login`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ email }) }).catch(() => null);
    if (!r?.ok) return bad("Could not sign in", 400);
    (await cookies()).set(DEV_COOKIE, (await r.json()).token, { httpOnly: true, sameSite: "lax", path: "/", maxAge: 86400 * 30 });
    return Response.json({ ok: true });
  }
  const { error } = await (await supabaseServer()).auth.signInWithPassword({ email, password });
  return error ? bad("Invalid email or password", 401) : Response.json({ ok: true });
}
