import { AD_QUEUE, getBoss } from "../src/lib/queue";
import { processAd } from "../src/lib/jobs";
import { migrate } from "../src/lib/migrate";

async function main() {
  await migrate();
  const boss = await getBoss();
  await boss.work<{ adId: string }>(AD_QUEUE, { localConcurrency: Number(process.env.WORKER_CONCURRENCY ?? 5) }, async (jobs) => {
    for (const j of jobs) await processAd(j.data.adId);
  });
  console.log("worker ready");
  const stop = async () => { await boss.stop({ graceful: true }); process.exit(0); };
  process.on("SIGTERM", stop);
  process.on("SIGINT", stop);
}

main().catch((e) => { console.error(e); process.exit(1); });
