import { randomUUID } from "crypto";
import { currentUser, quotaOf } from "@/lib/auth";
import { withDB, type Project } from "@/lib/db";
import { scrapeProduct } from "@/lib/scrape";
import { generateCopy } from "@/lib/script";
import { provider } from "@/lib/video";

export async function POST(req: Request) {
  const user = await currentUser();
  if (!user) return Response.json({ error: "Not signed in" }, { status: 401 });
  const body = await req.json();
  const url = String(body.url ?? "").trim();
  const remaining = quotaOf(user) - user.usedThisMonth;
  const count = Math.min(Math.max(Number(body.count) || 1, 1), 3);
  if (remaining < count) return Response.json({ error: `Not enough videos left this month (${remaining} remaining). Upgrade your plan.` }, { status: 402 });

  let product;
  try { product = await scrapeProduct(url); } catch (e) { return Response.json({ error: (e as Error).message }, { status: 400 }); }

  const copies = await generateCopy(product, count);
  const ads: Project["ads"] = await Promise.all(copies.map(async (c) => {
    try { return { ...c, status: "processing" as const, videoJobId: await provider.start({ ...c, status: "pending" }, product) }; }
    catch (e) { return { ...c, status: "failed" as const, error: (e as Error).message }; }
  }));

  const project: Project = { id: randomUUID(), userId: user.id, url, product, ads, createdAt: new Date().toISOString() };
  const charged = ads.filter((a) => a.status !== "failed").length;
  await withDB((db) => {
    db.projects.push(project);
    const u = db.users.find((x) => x.id === user.id)!;
    u.usedThisMonth += charged;
  });
  return Response.json({ id: project.id });
}
