import Stripe from "stripe";
import type { PlanId } from "./plans";

export const stripe = () => new Stripe(process.env.STRIPE_SECRET_KEY!);
export const billingEnabled = () => !!process.env.STRIPE_SECRET_KEY;

export const priceIdFor = (plan: PlanId) => process.env[`STRIPE_PRICE_${plan.toUpperCase()}`];
export const planForPrice = (priceId?: string): PlanId =>
  (["starter", "growth", "scale"] as const).find((p) => priceIdFor(p) === priceId) ?? "free";
