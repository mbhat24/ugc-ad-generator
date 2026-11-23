import { test, after } from "node:test";
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import Stripe from "stripe";
import { pool, query } from "../src/lib/db";
import { migrate } from "../src/lib/migrate";

const hasDb = !!process.env.DATABASE_URL;
after(() => pool.end());

test("stripe webhook: signature, plan sync and event dedupe", { skip: !hasDb }, async () => {
  Object.assign(process.env, { STRIPE_SECRET_KEY: "sk_test_x", STRIPE_WEBHOOK_SECRET: "whsec_test", STRIPE_PRICE_GROWTH: "price_growth", STRIPE_PRICE_STARTER: "price_starter", STRIPE_PRICE_SCALE: "price_scale" });
  await migrate();
  const { POST } = await import("../src/app/api/stripe/webhook/route");
  const customer = `cus_${randomUUID()}`;
  const [u] = await query<{ id: string }>("insert into profiles (id, email, stripe_customer_id) values ($1, $2, $3) returning id", [randomUUID(), `s${Date.now()}@test.dev`, customer]);
  const event = { id: `evt_${randomUUID()}`, object: "event", type: "customer.subscription.updated", data: { object: { id: "sub_1", object: "subscription", customer, status: "active", items: { data: [{ price: { id: "price_growth" } }] } } } };
  const payload = JSON.stringify(event);
  const send = (header: string) => POST(new Request("http://x/api/stripe/webhook", { method: "POST", body: payload, headers: { "stripe-signature": header } }));
  const good = new Stripe("sk_test_x").webhooks.generateTestHeaderString({ payload, secret: "whsec_test" });

  assert.equal((await send("t=1,v1=bad")).status, 400);
  assert.equal((await send(good)).status, 200);
  assert.equal((await query<{ plan: string }>("select plan from profiles where id = $1", [u.id]))[0].plan, "growth");
  assert.equal(((await (await send(good)).json()) as { duplicate?: boolean }).duplicate, true);

  const cancel = JSON.stringify({ ...event, id: `evt_${randomUUID()}`, type: "customer.subscription.deleted", data: { object: { ...event.data.object, status: "canceled" } } });
  const res = await POST(new Request("http://x/api/stripe/webhook", { method: "POST", body: cancel, headers: { "stripe-signature": new Stripe("sk_test_x").webhooks.generateTestHeaderString({ payload: cancel, secret: "whsec_test" }) } }));
  assert.equal(res.status, 200);
  assert.equal((await query<{ plan: string }>("select plan from profiles where id = $1", [u.id]))[0].plan, "free");
});
