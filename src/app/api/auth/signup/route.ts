import { signup } from "@/lib/auth";

export async function POST(req: Request) {
  const { email, password } = await req.json();
  try { await signup(String(email ?? ""), String(password ?? "")); return Response.json({ ok: true }); }
  catch (e) { return Response.json({ error: (e as Error).message }, { status: 400 }); }
}
