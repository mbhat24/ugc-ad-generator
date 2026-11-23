import { supabaseServer } from "@/lib/supabase";
import { devAuth, devLogin } from "@/lib/devauth";

export async function POST(req: Request) {
  const { email, password } = await req.json();
  if (devAuth) { await devLogin(String(email ?? "").trim()); return Response.json({ ok: true }); }
  const { error } = await (await supabaseServer()).auth.signInWithPassword({ email: String(email ?? "").trim(), password: String(password ?? "") });
  return error ? Response.json({ error: "Invalid email or password" }, { status: 401 }) : Response.json({ ok: true });
}
