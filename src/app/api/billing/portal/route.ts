import { currentUser } from "@/lib/auth";
import { badOrigin, fail } from "@/lib/http";
import { limited } from "@/lib/ratelimit";
import { query } from "@/lib/db";
import { billingEnabled, stripe } from "@/lib/stripe";

export async function POST(req: Request) {
  if (badOrigin(req)) return fail("Bad origin", 403);
  const user = await currentUser();
  if (!user) return fail("Not signed in", 401);
  if (await limited(req, "billing", 20, 3600, user.id)) return fail("Too many requests", 429);
  if (!billingEnabled()) return Response.json({ error: "Billing is not configured." }, { status: 501 });
  const customer = (await query<{ c: string | null }>("select stripe_customer_id c from profiles where id = $1", [user.id]))[0]?.c;
  if (!customer) return Response.json({ error: "No billing account yet" }, { status: 400 });
  const session = await stripe().billingPortal.sessions.create({ customer, return_url: `${process.env.APP_URL ?? new URL(req.url).origin}/dashboard` });
  return Response.json({ url: session.url });
}
