import { supabaseServer } from "@/lib/supabase";
import { devAuth, devLogout } from "@/lib/devauth";
import { badOrigin, fail } from "@/lib/http";

export async function POST(req: Request) {
  if (badOrigin(req)) return fail("Bad origin", 403);
  if (devAuth) await devLogout(); else await (await supabaseServer()).auth.signOut();
  return Response.json({ ok: true });
}
