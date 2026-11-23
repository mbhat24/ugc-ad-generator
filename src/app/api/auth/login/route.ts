import { supabaseServer } from "@/lib/supabase";

export async function POST(req: Request) {
  const { email, password } = await req.json();
  const { error } = await (await supabaseServer()).auth.signInWithPassword({ email: String(email ?? "").trim(), password: String(password ?? "") });
  return error ? Response.json({ error: "Invalid email or password" }, { status: 401 }) : Response.json({ ok: true });
}
