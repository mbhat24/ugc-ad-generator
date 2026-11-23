import { login } from "@/lib/auth";

export async function POST(req: Request) {
  const { email, password } = await req.json();
  try { await login(String(email ?? ""), String(password ?? "")); return Response.json({ ok: true }); }
  catch (e) { return Response.json({ error: (e as Error).message }, { status: 401 }); }
}
