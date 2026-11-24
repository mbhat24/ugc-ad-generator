// Usage: BASE=http://localhost:8080 USERS=25 node tools/loadtest.mjs   (API in dev-auth mode, mock provider)
const BASE = process.env.BASE ?? "http://localhost:8080";
const USERS = Number(process.env.USERS ?? 20);
const PRODUCT = process.env.PRODUCT_URL ?? "https://example.com";
const j = { "content-type": "application/json" };
const pct = (a, p) => [...a].sort((x, y) => x - y)[Math.min(a.length - 1, Math.floor(a.length * p))];

async function one(i) {
  const email = `load${Date.now()}_${i}@test.dev`;
  const { token } = await (await fetch(`${BASE}/v1/dev/login`, { method: "POST", headers: j, body: JSON.stringify({ email }) })).json();
  const h = { ...j, authorization: `Bearer ${token}` };
  const t0 = performance.now();
  const r = await fetch(`${BASE}/v1/projects`, { method: "POST", headers: h, body: JSON.stringify({ url: PRODUCT, count: 1 }) });
  const create = performance.now() - t0;
  if (!r.ok) return { ok: false, status: r.status, create };
  const { id } = await r.json();
  for (let k = 0; k < 150; k++) {
    const p = await (await fetch(`${BASE}/v1/projects/${id}`, { headers: h })).json();
    const st = p.ads[0].status;
    if (st === "done" || st === "failed") return { ok: st === "done", status: 200, create, total: performance.now() - t0 };
    await new Promise((res) => setTimeout(res, 2000));
  }
  return { ok: false, status: 0, create, total: Infinity };
}

const res = await Promise.all(Array.from({ length: USERS }, (_, i) => one(i)));
const ok = res.filter((r) => r.ok);
console.log(JSON.stringify({ users: USERS, ok: ok.length, failed: res.length - ok.length, statuses: [...new Set(res.filter((r) => !r.ok).map((r) => r.status))], createMs: { p50: Math.round(pct(res.map((r) => r.create), 0.5)), p95: Math.round(pct(res.map((r) => r.create), 0.95)) }, endToEndMs: ok.length ? { p50: Math.round(pct(ok.map((r) => r.total), 0.5)), p95: Math.round(pct(ok.map((r) => r.total), 0.95)) } : null }, null, 1));
