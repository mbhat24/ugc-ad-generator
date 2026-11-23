import { currentUser } from "@/lib/auth";
import { query } from "@/lib/db";
import { billingEnabled, stripe } from "@/lib/stripe";

export async function POST(req: Request) {
  const user = await currentUser();
  if (!user) return Response.json({ error: "Not signed in" }, { status: 401 });
  if (!billingEnabled()) return Response.json({ error: "Billing is not configured." }, { status: 501 });
  const customer = (await query<{ c: string | null }>("select stripe_customer_id c from profiles where id = $1", [user.id]))[0]?.c;
  if (!customer) return Response.json({ error: "No billing account yet" }, { status: 400 });
  const session = await stripe().billingPortal.sessions.create({ customer, return_url: `${process.env.APP_URL ?? new URL(req.url).origin}/dashboard` });
  return Response.json({ url: session.url });
}
