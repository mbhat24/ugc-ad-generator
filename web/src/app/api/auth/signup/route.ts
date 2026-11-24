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
  if (!/^\S+@\S+\.\S+$/.test(email) || email.length > 254 || password.length < 8 || password.length > 128) return bad("Enter a valid email and a password of 8+ characters", 400);

  if (devAuth) {
    const r = await fetch(`${API_URL}/v1/dev/login`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ email }) }).catch(() => null);
    if (!r?.ok) return bad("Could not sign up", 400);
    (await cookies()).set(DEV_COOKIE, (await r.json()).token, { httpOnly: true, sameSite: "lax", path: "/", maxAge: 86400 * 30 });
    return Response.json({ ok: true });
  }
  const { data, error } = await (await supabaseServer()).auth.signUp({ email, password, options: { emailRedirectTo: `${process.env.APP_URL ?? ""}/dashboard` } });
  if (error) return bad(error.message, 400);
  return Response.json({ ok: true, confirm: !data.session });
}
