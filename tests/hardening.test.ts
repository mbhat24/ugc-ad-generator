import { test, after } from "node:test";
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { pool, query } from "../src/lib/db";
import { migrate } from "../src/lib/migrate";
import { stopBoss } from "../src/lib/queue";
import { hit } from "../src/lib/ratelimit";
import { reconcile } from "../src/lib/jobs";
import { configProblems } from "../src/lib/env";
import { badOrigin } from "../src/lib/http";

const hasDb = !!process.env.DATABASE_URL;
after(async () => { await stopBoss(); await pool.end(); });

test("migrations are idempotent", { skip: !hasDb }, async () => {
  await migrate();
  await migrate();
  assert.ok((await query("select 1 from schema_migrations where name = '002_reliability_and_compliance.sql'")).length === 1);
});

test("rate limiter blocks after max within window", { skip: !hasDb }, async () => {
  const key = `t:${randomUUID()}`;
  const results = [];
  for (let i = 0; i < 4; i++) results.push(await hit(key, 3, 60));
  assert.deepEqual(results, [false, false, false, true]);
});

test("reconciler finds stale active ads but ignores fresh and finished ones", { skip: !hasDb }, async () => {
  await migrate();
  const [u] = await query<{ id: string }>("insert into profiles (id, email) values ($1, $2) returning id", [randomUUID(), `r${Date.now()}@test.dev`]);
  const [p] = await query<{ id: string }>("insert into projects (user_id, url, product) values ($1, 'x', '{}') returning id", [u.id]);
  const mk = async (status: string, ageMin: number) => (await query<{ id: string }>("insert into ads (project_id, position, hook, script, caption, avatar, status, updated_at) values ($1, 0, 'h', 's', 'c', 'a', $2, now() - $3 * interval '1 minute') returning id", [p.id, status, ageMin]))[0].id;
  await mk("processing", 10); await mk("pending", 10); await mk("processing", 0); await mk("done", 10);
  const r = await reconcile();
  assert.ok(r.requeued >= 2);
});

test("config validation reports missing and partial settings", () => {
  const problems = configProblems({ STRIPE_SECRET_KEY: "sk" } as unknown as NodeJS.ProcessEnv);
  assert.ok(problems.some((p) => p.includes("Stripe is partly configured")));
  assert.deepEqual(configProblems({} as unknown as NodeJS.ProcessEnv), process.env.NODE_ENV === "production" ? configProblems({} as unknown as NodeJS.ProcessEnv) : []);
});

test("origin check rejects foreign origins", () => {
  const r = (origin?: string) => new Request("http://app.test/api", { method: "POST", headers: { host: "app.test", ...(origin ? { origin } : {}) } });
  assert.equal(badOrigin(r("http://app.test")), false);
  assert.equal(badOrigin(r()), false);
  assert.equal(badOrigin(r("https://evil.example")), true);
});
