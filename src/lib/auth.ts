import { balanceOf, query } from "./db";
import { PLANS, type PlanId } from "./plans";
import { supabaseServer } from "./supabase";

export type User = { id: string; email: string; plan: PlanId; balance: number; hasBilling: boolean };

export async function currentUser(): Promise<User | null> {
  const { data } = await (await supabaseServer()).auth.getUser();
  const u = data.user;
  if (!u?.email) return null;
  const rows = await query<{ plan: PlanId; stripe_customer_id: string | null; created: boolean }>(
    `insert into profiles (id, email) values ($1, $2)
     on conflict (id) do update set email = excluded.email
     returning plan, stripe_customer_id, (xmax = 0) as created`, [u.id, u.email]);
  const p = rows[0];
  if (p.created) await query("insert into credit_ledger (user_id, delta, reason, ref) values ($1, $2, 'signup', $1::text) on conflict do nothing", [u.id, PLANS.free.videos]);
  return { id: u.id, email: u.email, plan: p.plan, balance: await balanceOf(u.id), hasBilling: !!p.stripe_customer_id };
}
