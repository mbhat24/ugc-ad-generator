import { supabaseServer } from "@/lib/supabase";
import { devAuth, devLogin } from "@/lib/devauth";
import { badOrigin, fail, readJson } from "@/lib/http";
import { limited } from "@/lib/ratelimit";

export async function POST(req: Request) {
  if (badOrigin(req)) return fail("Bad origin", 403);
  const body = await readJson(req);
  if (!body) return fail("Invalid request", 400);
  const e = String(body.email ?? "").trim().toLowerCase();
  const password = String(body.password ?? "");
  if (!/^\S+@\S+\.\S+$/.test(e) || e.length > 254 || password.length < 8 || password.length > 128) return fail("Enter a valid email and a password of 8+ characters", 400);
  if (await limited(req, "signup", Number(process.env.SIGNUPS_PER_IP_HOUR ?? 5), 3600)) return fail("Too many signups from this network. Try again later.", 429);
  if (devAuth) { await devLogin(e); return Response.json({ ok: true }); }
  const { data, error } = await (await supabaseServer()).auth.signUp({ email: e, password, options: { emailRedirectTo: `${process.env.APP_URL ?? ""}/dashboard` } });
  if (error) return fail(error.message, 400);
  return Response.json({ ok: true, confirm: !data.session });
}
