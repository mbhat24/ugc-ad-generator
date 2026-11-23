import { pool } from "./db";
import { SCHEMA } from "./schema";

export async function migrate() {
  const c = await pool.connect();
  try {
    await c.query("select pg_advisory_lock(727001)");
    await c.query(SCHEMA);
  } finally {
    await c.query("select pg_advisory_unlock(727001)").catch(() => {});
    c.release();
  }
}
