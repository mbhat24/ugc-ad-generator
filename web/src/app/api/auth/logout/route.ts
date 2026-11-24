import { cookies } from "next/headers";
import { DEV_COOKIE, devAuth } from "@/lib/devauth";
import { supabaseServer } from "@/lib/supabase";
import { sameOrigin } from "@/lib/origin";

export async function POST(req: Request) {
  if (!sameOrigin(req)) return Response.json({ error: "Bad origin" }, { status: 403 });
  if (devAuth) (await cookies()).delete(DEV_COOKIE);
  else await (await supabaseServer()).auth.signOut();
  return Response.json({ ok: true });
}
