import { currentUser } from "@/lib/auth";
import { tx } from "@/lib/db";
import { enqueueAd } from "@/lib/queue";
import { limited, overUsage } from "@/lib/ratelimit";
import { badOrigin, fail, readJson } from "@/lib/http";
import { log } from "@/lib/log";
import { scrapeProduct } from "@/lib/scrape";
import { generateCopy } from "@/lib/script";

export async function POST(req: Request) {
  if (badOrigin(req)) return fail("Bad origin", 403);
  const user = await currentUser();
  if (!user) return fail("Not signed in", 401);
  if (await limited(req, "projects", 30, 3600, user.id)) return fail("Too many requests. Try again later.", 429);
  const body = (await readJson(req)) ?? {};
  const url = String(body.url ?? "").trim().slice(0, 2000);
  const count = Math.min(Math.max(Number(body.count) || 1, 1), 3);
  if (user.balance < count) return Response.json({ error: `Not enough videos left (${user.balance} remaining). Upgrade your plan.`, code: "no_credits" }, { status: 402 });
  const overQuota = await overUsage(user.id);
  if (overQuota) return fail(overQuota, 429);

  let product;
  try { product = await scrapeProduct(url); } catch (e) { return Response.json({ error: (e as Error).message }, { status: 400 }); }
  const copies = await generateCopy(product, count);

  const created = await tx(async (c) => {
    await c.query("select 1 from profiles where id = $1 for update", [user.id]);
    const bal = Number((await c.query("select coalesce(sum(delta),0) b from credit_ledger where user_id = $1", [user.id])).rows[0].b);
    if (bal < count) return null;
    const pid = (await c.query("insert into projects (user_id, url, product) values ($1,$2,$3) returning id", [user.id, url, product])).rows[0].id as string;
    const adIds: string[] = [];
    for (const [i, a] of copies.entries()) {
      const r = await c.query("insert into ads (project_id, position, hook, script, caption, hashtags, avatar) values ($1,$2,$3,$4,$5,$6,$7) returning id", [pid, i, a.hook, a.script, a.caption, JSON.stringify(a.hashtags), a.avatar]);
      adIds.push(r.rows[0].id);
      await c.query("insert into credit_ledger (user_id, delta, reason, ref) values ($1, -1, 'ad', $2)", [user.id, r.rows[0].id]);
    }
    return { pid, adIds };
  });
  if (!created) return Response.json({ error: "Not enough videos left. Upgrade your plan.", code: "no_credits" }, { status: 402 });

  log.info("project created", { userId: user.id, projectId: created.pid, ads: created.adIds.length });
  try { await Promise.all(created.adIds.map((id) => enqueueAd(id))); }
  catch (e) {
    log.error("enqueue failed, ads will be picked up by reconciler", { projectId: created.pid, err: e });
    return Response.json({ id: created.pid });
  }
  return Response.json({ id: created.pid });
}
