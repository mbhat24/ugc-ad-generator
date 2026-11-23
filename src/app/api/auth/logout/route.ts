import { supabaseServer } from "@/lib/supabase";

export async function POST() {
  await (await supabaseServer()).auth.signOut();
  return Response.json({ ok: true });
}
