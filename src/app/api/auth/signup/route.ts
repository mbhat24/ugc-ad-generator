import { supabaseServer } from "@/lib/supabase";
import { devAuth, devLogin } from "@/lib/devauth";

export async function POST(req: Request) {
  const { email, password } = await req.json();
  const e = String(email ?? "").trim().toLowerCase();
  if (!/^\S+@\S+\.\S+$/.test(e) || String(password ?? "").length < 8) return Response.json({ error: "Enter a valid email and a password of 8+ characters" }, { status: 400 });
  if (devAuth) { await devLogin(e); return Response.json({ ok: true }); }
  const { data, error } = await (await supabaseServer()).auth.signUp({ email: e, password: String(password), options: { emailRedirectTo: `${process.env.APP_URL ?? ""}/dashboard` } });
  if (error) return Response.json({ error: error.message }, { status: 400 });
  return Response.json({ ok: true, confirm: !data.session });
}
