import { PgBoss } from "pg-boss";
import { log } from "./log";

export const AD_QUEUE = "ad-generate";
export const AD_DLQ = "ad-generate-dlq";
export const RECONCILE_QUEUE = "reconcile";
const g = globalThis as unknown as { boss?: Promise<PgBoss> };

const ssl = (process.env.DATABASE_SSL ?? (process.env.NODE_ENV === "production" ? "require" : "off")) === "off" ? undefined : { rejectUnauthorized: process.env.DATABASE_SSL === "verify", ca: process.env.DATABASE_CA };

export function getBoss() {
  return (g.boss ??= (async () => {
    const boss = new PgBoss({ connectionString: process.env.DATABASE_URL, ssl, max: Number(process.env.BOSS_POOL_MAX ?? 5), application_name: "adreel-queue" });
    boss.on("error", (e) => log.error("pg-boss", { err: e }));
    await boss.start();
    await boss.createQueue(AD_DLQ);
    await boss.createQueue(AD_QUEUE, { retryLimit: 3, retryDelay: 5, retryBackoff: true, expireInSeconds: 900, deadLetter: AD_DLQ });
    await boss.createQueue(RECONCILE_QUEUE);
    return boss;
  })().catch((e) => { g.boss = undefined; throw e; }));
}

export async function enqueueAd(adId: string, startAfter = 0, dedupe = false) {
  await (await getBoss()).send(AD_QUEUE, { adId }, { startAfter, ...(dedupe ? { singletonKey: adId, singletonSeconds: 120 } : {}) });
}

export async function stopBoss() {
  if (!g.boss) return;
  const b = await g.boss.catch(() => null);
  g.boss = undefined;
  await b?.stop({ graceful: false, timeout: 5000 }).catch(() => {});
}
