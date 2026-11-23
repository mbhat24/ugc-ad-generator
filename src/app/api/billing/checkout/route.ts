import { currentUser } from "@/lib/auth";
import { query } from "@/lib/db";
import { PLANS, type PlanId } from "@/lib/plans";
import { billingEnabled, priceIdFor, stripe } from "@/lib/stripe";

export async function POST(req: Request) {
  const user = await currentUser();
  if (!user) return Response.json({ error: "Not signed in" }, { status: 401 });
  if (!billingEnabled()) return Response.json({ error: "Billing is not configured." }, { status: 501 });
  const { plan } = await req.json();
  const price = plan !== "free" && plan in PLANS ? priceIdFor(plan as PlanId) : undefined;
  if (!price) return Response.json({ error: "Unknown plan" }, { status: 400 });

  const s = stripe();
  let customer = (await query<{ c: string | null }>("select stripe_customer_id c from profiles where id = $1", [user.id]))[0]?.c;
  if (!customer) {
    customer = (await s.customers.create({ email: user.email, metadata: { userId: user.id } })).id;
    await query("update profiles set stripe_customer_id = $2 where id = $1", [user.id, customer]);
  }
  const base = process.env.APP_URL ?? new URL(req.url).origin;
  const session = await s.checkout.sessions.create({
    mode: "subscription", customer, client_reference_id: user.id,
    line_items: [{ price, quantity: 1 }],
    success_url: `${base}/dashboard?upgraded=1`, cancel_url: `${base}/dashboard`,
  });
  return Response.json({ url: session.url });
}
