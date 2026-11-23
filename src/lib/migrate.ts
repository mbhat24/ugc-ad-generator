import { createHash } from "crypto";
import { readdir, readFile } from "fs/promises";
import { join } from "path";
import { pool } from "./db";
import { log } from "./log";

const DIR = join(process.cwd(), "migrations");

export async function migrate() {
  const files = (await readdir(DIR)).filter((f) => f.endsWith(".sql")).sort();
  const c = await pool.connect();
  try {
    await c.query("select pg_advisory_lock(727001)");
    await c.query("create table if not exists schema_migrations (name text primary key, checksum text not null, applied_at timestamptz not null default now())");
    const applied = new Map((await c.query<{ name: string; checksum: string }>("select name, checksum from schema_migrations")).rows.map((r) => [r.name, r.checksum]));
    for (const f of files) {
      const sql = await readFile(join(DIR, f), "utf8");
      const checksum = createHash("sha256").update(sql).digest("hex");
      if (applied.has(f)) {
        if (applied.get(f) !== checksum) throw new Error(`Migration ${f} was modified after being applied`);
        continue;
      }
      try {
        await c.query("begin");
        await c.query(sql);
        await c.query("insert into schema_migrations (name, checksum) values ($1, $2)", [f, checksum]);
        await c.query("commit");
        log.info("migration applied", { name: f });
      } catch (e) {
        await c.query("rollback").catch(() => {});
        throw e;
      }
    }
  } finally {
    await c.query("select pg_advisory_unlock(727001)").catch(() => {});
    c.release();
  }
}
