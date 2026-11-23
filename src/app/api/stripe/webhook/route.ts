import type Stripe from "stripe";
import { query } from "@/lib/db";
import { PLANS } from "@/lib/plans";
import { planForPrice, stripe } from "@/lib/stripe";
import { log } from "@/lib/log";

const customerId = (c: string | Stripe.Customer | Stripe.DeletedCustomer | null) => (typeof c === "string" ? c : c?.id);

async function applySubscription(sub: Stripe.Subscription) {
  const active = ["active", "trialing", "past_due"].includes(sub.status);
  const plan = active ? planForPrice(sub.items.data[0]?.price.id) : "free";
  await query("update profiles set plan = $2 where stripe_customer_id = $1", [customerId(sub.customer), plan]);
}

export async function POST(req: Request) {
  const sig = req.headers.get("stripe-signature");
  let event: Stripe.Event;
  try { event = stripe().webhooks.constructEvent(await req.text(), sig ?? "", process.env.STRIPE_WEBHOOK_SECRET!); }
  catch { return new Response("Bad signature", { status: 400 }); }

  const fresh = await query("insert into stripe_events (id, type) values ($1, $2) on conflict do nothing returning id", [event.id, event.type]);
  if (!fresh.length) return Response.json({ received: true, duplicate: true });
  try { await handle(event); }
  catch (e) {
    await query("delete from stripe_events where id = $1", [event.id]).catch(() => {});
    log.error("stripe webhook failed", { type: event.type, eventId: event.id, err: e });
    return new Response("Handler error", { status: 500 });
  }
  log.info("stripe event handled", { type: event.type, eventId: event.id });
  return Response.json({ received: true });
}

async function handle(event: Stripe.Event) {
  switch (event.type) {
    case "customer.subscription.created":
    case "customer.subscription.updated":
    case "customer.subscription.deleted":
      await applySubscription(event.data.object);
      break;
    case "invoice.paid": {
      const inv = event.data.object;
      const subId = inv.parent?.subscription_details?.subscription;
      if (!subId) break;
      const sub = await stripe().subscriptions.retrieve(typeof subId === "string" ? subId : subId.id);
      await applySubscription(sub);
      const plan = planForPrice(sub.items.data[0]?.price.id);
      if (plan === "free") break;
      await query(
        `insert into credit_ledger (user_id, delta, reason, ref)
         select id, $2, 'subscription', $3 from profiles where stripe_customer_id = $1 on conflict do nothing`,
        [customerId(inv.customer), PLANS[plan].videos, inv.id]);
      break;
    }
    case "invoice.payment_failed":
      log.warn("invoice payment failed", { customer: customerId(event.data.object.customer) });
      break;
  }
}
