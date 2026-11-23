import { test, after } from "node:test";
import assert from "node:assert/strict";
import { createHmac, randomUUID } from "node:crypto";
import { pool, query } from "../src/lib/db";
import { migrate } from "../src/lib/migrate";
import { stopBoss } from "../src/lib/queue";
import { processAd } from "../src/lib/jobs";

const BASE = process.env.TEST_BASE_URL ?? "http://localhost:3000";
const hasDb = !!process.env.DATABASE_URL;
after(async () => { await stopBoss(); await pool.end(); });

async function seedAd(ageMinutes = 0) {
  const [u] = await query<{ id: string }>("insert into profiles (id, email) values ($1, $2) returning id", [randomUUID(), `t${Date.now()}${Math.random()}@test.dev`]);
  const [p] = await query<{ id: string }>("insert into projects (user_id, url, product) values ($1, 'x', $2) returning id", [u.id, JSON.stringify({ title: "t", description: "", images: [] })]);
  const [a] = await query<{ id: string }>("insert into ads (project_id, position, hook, script, caption, avatar, created_at) values ($1, 0, 'h', 's', 'c', 'a', now() - $2 * interval '1 minute') returning id", [p.id, ageMinutes]);
  await query("insert into credit_ledger (user_id, delta, reason, ref) values ($1, -1, 'ad', $2)", [u.id, a.id]);
  return { userId: u.id, adId: a.id };
}

test("timed-out ad is failed and refunded exactly once", { skip: !hasDb }, async () => {
  await migrate();
  const { adId } = await seedAd(60);
  await Promise.all([processAd(adId), processAd(adId), processAd(adId)]);
  const [ad] = await query<{ status: string }>("select status from ads where id = $1", [adId]);
  const refunds = await query("select 1 from credit_ledger where reason = 'refund' and ref = $1", [adId]);
  assert.equal(ad.status, "failed");
  assert.equal(refunds.length, 1);
});

test("ledger rejects duplicate idempotency keys", { skip: !hasDb }, async () => {
  await migrate();
  const { userId, adId } = await seedAd();
  await query("insert into credit_ledger (user_id, delta, reason, ref) values ($1, 1, 'refund', $2) on conflict do nothing", [userId, adId]);
  await query("insert into credit_ledger (user_id, delta, reason, ref) values ($1, 1, 'refund', $2) on conflict do nothing", [userId, adId]);
  assert.equal((await query("select 1 from credit_ledger where reason = 'refund' and ref = $1", [adId])).length, 1);
});

const sign = (secret: string, id: string, ts: string, body: string) =>
  `v1,${createHmac("sha256", Buffer.from(secret.replace(/^whsec_/, ""), "base64")).update(`${id}.${ts}.${body}`).digest("base64")}`;

test("higgsfield webhook verifies signatures", { skip: !hasDb }, async () => {
  process.env.HIGGSFIELD_WEBHOOK_SECRET = `whsec_${Buffer.from("test-secret").toString("base64")}`;
  const { POST } = await import("../src/app/api/higgsfield/webhook/route");
  const body = JSON.stringify({ request_id: randomUUID(), status: "completed" });
  const call = (sig: string, ts = String(Math.floor(Date.now() / 1000))) =>
    POST(new Request("http://x/api", { method: "POST", body, headers: { "webhook-id": "evt1", "webhook-timestamp": ts, "webhook-signature": sig } }));
  const ts = String(Math.floor(Date.now() / 1000));
  assert.equal((await call(sign(process.env.HIGGSFIELD_WEBHOOK_SECRET!, "evt1", ts, body), ts)).status, 200);
  assert.equal((await call("v1,AAAA", ts)).status, 400);
  const old = String(Math.floor(Date.now() / 1000) - 3600);
  assert.equal((await call(sign(process.env.HIGGSFIELD_WEBHOOK_SECRET!, "evt1", old, body), old)).status, 400);
});

test("concurrent project creation never overdraws credits (needs running dev server)", async (t) => {
  if (!(await fetch(BASE, { signal: AbortSignal.timeout(2000) }).then(() => true, () => false))) return t.skip("server not running");
  const email = `race${Date.now()}@test.dev`;
  const r = await fetch(`${BASE}/api/auth/signup`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ email, password: "password123" }) });
  const cookie = r.headers.getSetCookie().map((c) => c.split(";")[0]).join("; ");
  if (!cookie) return t.skip("auth not in dev mode");
  const res = await Promise.all(Array.from({ length: 5 }, () => fetch(`${BASE}/api/projects`, { method: "POST", headers: { "content-type": "application/json", cookie }, body: JSON.stringify({ url: "https://example.com", count: 1 }) })));
  const codes = res.map((x) => x.status).sort();
  assert.equal(codes.filter((c) => c === 200).length, 1, `statuses: ${codes}`);
});
