import type Stripe from "stripe";
import { query } from "@/lib/db";
import { PLANS } from "@/lib/plans";
import { planForPrice, stripe } from "@/lib/stripe";

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
  }
  return Response.json({ received: true });
}
