import { migrate } from "../src/lib/migrate";
import { pool } from "../src/lib/db";

migrate().then(() => { console.log("migrated"); return pool.end(); }).catch((e) => { console.error(e); process.exit(1); });
