import { supabaseServer } from "@/lib/supabase";
import { devAuth, devLogout } from "@/lib/devauth";

export async function POST() {
  if (devAuth) await devLogout(); else await (await supabaseServer()).auth.signOut();
  return Response.json({ ok: true });
}
