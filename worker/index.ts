import { AD_DLQ, AD_QUEUE, RECONCILE_QUEUE, getBoss } from "../src/lib/queue";
import { failFromDeadLetter, processAd, reconcile } from "../src/lib/jobs";
import { migrate } from "../src/lib/migrate";
import { assertConfig } from "../src/lib/env";
import { pool } from "../src/lib/db";
import { log } from "../src/lib/log";

async function main() {
  assertConfig();
  await migrate();
  const boss = await getBoss();
  await boss.work<{ adId: string }>(AD_QUEUE, { localConcurrency: Number(process.env.WORKER_CONCURRENCY ?? 5) }, async (jobs) => {
    for (const j of jobs) await processAd(j.data.adId);
  });
  await boss.work<{ adId: string }>(AD_DLQ, async (jobs) => {
    for (const j of jobs) await failFromDeadLetter(j.data.adId);
  });
  await boss.work(RECONCILE_QUEUE, async () => { await reconcile(); });
  await boss.schedule(RECONCILE_QUEUE, "* * * * *");
  log.info("worker ready");

  let stopping = false;
  const stop = async (sig: string) => {
    if (stopping) return;
    stopping = true;
    log.info("worker stopping", { sig });
    const t = setTimeout(() => process.exit(1), 30000);
    await boss.stop({ graceful: true }).catch(() => {});
    await pool.end().catch(() => {});
    clearTimeout(t);
    process.exit(0);
  };
  process.on("SIGTERM", () => stop("SIGTERM"));
  process.on("SIGINT", () => stop("SIGINT"));
}

process.on("unhandledRejection", (e) => log.error("unhandledRejection", { err: e }));
process.on("uncaughtException", (e) => { log.error("uncaughtException", { err: e }); process.exit(1); });
main().catch((e) => { log.error("worker failed to start", { err: e }); process.exit(1); });
