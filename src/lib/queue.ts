import { PgBoss } from "pg-boss";

export const AD_QUEUE = "ad-generate";
const g = globalThis as unknown as { boss?: Promise<PgBoss> };

export function getBoss() {
  return (g.boss ??= (async () => {
    const boss = new PgBoss(process.env.DATABASE_URL!);
    boss.on("error", (e) => console.error("pg-boss", e));
    await boss.start();
    await boss.createQueue(AD_QUEUE, { retryLimit: 3, retryDelay: 5, retryBackoff: true, expireInSeconds: 900 });
    return boss;
  })());
}

export async function enqueueAd(adId: string, startAfter = 0) {
  await (await getBoss()).send(AD_QUEUE, { adId }, { startAfter });
}
