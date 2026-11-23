import { supabaseServer } from "@/lib/supabase";
import { devAuth, devLogin } from "@/lib/devauth";
import { badOrigin, fail, readJson } from "@/lib/http";
import { limited } from "@/lib/ratelimit";

export async function POST(req: Request) {
  if (badOrigin(req)) return fail("Bad origin", 403);
  const body = await readJson(req);
  if (!body) return fail("Invalid request", 400);
  const email = String(body.email ?? "").trim().toLowerCase();
  if (await limited(req, "login", 10, 60) || await limited(req, "login-email", 8, 300, email)) return fail("Too many attempts. Try again soon.", 429);
  if (devAuth) { await devLogin(email); return Response.json({ ok: true }); }
  const { error } = await (await supabaseServer()).auth.signInWithPassword({ email, password: String(body.password ?? "") });
  return error ? fail("Invalid email or password", 401) : Response.json({ ok: true });
}
