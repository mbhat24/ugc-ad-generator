import { createClient } from "@supabase/supabase-js";
import { currentUser } from "@/lib/auth";
import { query } from "@/lib/db";
import { devAuth, devLogout } from "@/lib/devauth";
import { badOrigin, fail, readJson } from "@/lib/http";
import { log } from "@/lib/log";
import { deletePrefix } from "@/lib/storage";
import { billingEnabled, stripe } from "@/lib/stripe";

export async function DELETE(req: Request) {
  if (badOrigin(req)) return fail("Bad origin", 403);
  const user = await currentUser();
  if (!user) return fail("Not signed in", 401);
  const body = await readJson(req);
  if (body?.confirm !== user.email) return fail("Type your email to confirm account deletion", 400);

  try {
    const customer = (await query<{ c: string | null }>("select stripe_customer_id c from profiles where id = $1", [user.id]))[0]?.c;
    if (customer && billingEnabled()) {
      const subs = await stripe().subscriptions.list({ customer, status: "all", limit: 20 });
      for (const s of subs.data) if (!["canceled", "incomplete_expired"].includes(s.status)) await stripe().subscriptions.cancel(s.id);
    }
    await deletePrefix(`videos/${user.id}/`);
    if (devAuth) await devLogout();
    else {
      if (!process.env.SUPABASE_SERVICE_ROLE_KEY) return fail("Account deletion is not configured", 501);
      const admin = createClient(process.env.NEXT_PUBLIC_SUPABASE_URL!, process.env.SUPABASE_SERVICE_ROLE_KEY);
      const { error } = await admin.auth.admin.deleteUser(user.id);
      if (error && !/not found/i.test(error.message)) throw error;
    }
    await query("delete from profiles where id = $1", [user.id]);
  } catch (e) {
    log.error("account deletion failed", { userId: user.id, err: e });
    return fail("Could not delete the account completely. Please retry or contact support.", 500);
  }
  log.info("account deleted", { userId: user.id });
  return Response.json({ ok: true });
}
