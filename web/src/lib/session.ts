import { cookies } from "next/headers";
import { DEV_COOKIE, devAuth } from "./devauth";
import { supabaseServer } from "./supabase";

export async function accessToken(): Promise<string | null> {
  if (devAuth) return (await cookies()).get(DEV_COOKIE)?.value ?? null;
  const { data } = await (await supabaseServer()).auth.getSession();
  return data.session?.access_token ?? null;
}
