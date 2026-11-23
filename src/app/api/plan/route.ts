import { currentUser } from "@/lib/auth";
import { withDB } from "@/lib/db";
import { PLANS, type PlanId } from "@/lib/plans";

export async function POST(req: Request) {
  const user = await currentUser();
  if (!user) return Response.json({ error: "Not signed in" }, { status: 401 });
  if (process.env.NODE_ENV === "production") return Response.json({ error: "Billing not configured. Connect Stripe Checkout." }, { status: 501 });
  const { plan } = await req.json();
  if (!(plan in PLANS)) return Response.json({ error: "Unknown plan" }, { status: 400 });
  await withDB((db) => { const u = db.users.find((x) => x.id === user.id)!; u.plan = plan as PlanId; u.usedThisMonth = 0; });
  return Response.json({ ok: true });
}
