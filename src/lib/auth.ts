import { balanceOf, query } from "./db";
import { PLANS, type PlanId } from "./plans";
import { supabaseServer } from "./supabase";
import { devAuth, devEmail, devId } from "./devauth";

export type User = { id: string; email: string; plan: PlanId; balance: number; hasBilling: boolean };

export async function currentUser(): Promise<User | null> {
  let u: { id: string; email?: string; email_confirmed_at?: string } | null;
  if (devAuth) { const e = await devEmail(); u = e ? { id: devId(e), email: e } : null; }
  else u = (await (await supabaseServer()).auth.getUser()).data.user;
  if (!u?.email) return null;
  if (!devAuth && process.env.NODE_ENV === "production" && !("email_confirmed_at" in u && u.email_confirmed_at)) return null;
  const [p] = await query<{ plan: PlanId; stripe_customer_id: string | null }>(
    `with p as (
       insert into profiles (id, email) values ($1::uuid, $2) on conflict (id) do update set email = excluded.email
       returning plan, stripe_customer_id, (xmax = 0) as created),
     g as (
       insert into credit_ledger (user_id, delta, reason, ref) select $1::uuid, $4::int, 'signup', $3::text from p where created on conflict do nothing)
     select plan, stripe_customer_id from p`, [u.id, u.email, u.id, PLANS.free.videos]);
  return { id: u.id, email: u.email, plan: p.plan, balance: await balanceOf(u.id), hasBilling: !!p.stripe_customer_id };
}
