import { Pool, type PoolClient, type QueryResultRow } from "pg";
import type { PlanId } from "./plans";
import { log } from "./log";

export type Ad = { id: string; hook: string; script: string; caption: string; hashtags: string[]; avatar: string; status: "pending" | "processing" | "done" | "failed"; videoJobId?: string; videoUrl?: string; error?: string };
export type Product = { title: string; description: string; price?: string; image?: string; images: string[] };
export type Project = { id: string; userId: string; url: string; product: Product; ads: Ad[]; createdAt: string };
export type Profile = { id: string; email: string; plan: PlanId; stripeCustomerId: string | null };

const g = globalThis as unknown as { pgPool?: Pool };
const sslMode = process.env.DATABASE_SSL ?? (process.env.NODE_ENV === "production" ? "require" : "off");
const ssl = sslMode === "off" ? undefined : { rejectUnauthorized: sslMode === "verify", ca: process.env.DATABASE_CA };

function createPool() {
  const p = new Pool({ connectionString: process.env.DATABASE_URL, ssl, max: Number(process.env.PG_POOL_MAX ?? 10), idleTimeoutMillis: 30000, connectionTimeoutMillis: 10000, statement_timeout: Number(process.env.PG_STATEMENT_TIMEOUT_MS ?? 15000), application_name: process.env.PG_APP_NAME ?? "adreel" });
  p.on("error", (e) => log.error("pg pool error", { err: e }));
  return p;
}

export const pool = (g.pgPool ??= createPool());

export async function query<T extends QueryResultRow = QueryResultRow>(text: string, params: unknown[] = []) {
  return (await pool.query<T>(text, params)).rows;
}

export async function tx<T>(fn: (c: PoolClient) => Promise<T>): Promise<T> {
  const c = await pool.connect();
  try {
    await c.query("begin");
    const r = await fn(c);
    await c.query("commit");
    return r;
  } catch (e) {
    await c.query("rollback").catch(() => {});
    throw e;
  } finally {
    c.release();
  }
}

type AdRow = { id: string; hook: string; script: string; caption: string; hashtags: string[]; avatar: string; status: Ad["status"]; video_job_id: string | null; video_url: string | null; error: string | null };
type ProjectRow = { id: string; user_id: string; url: string; product: Product; created_at: Date };

export const adFromRow = (r: AdRow): Ad => ({ id: r.id, hook: r.hook, script: r.script, caption: r.caption, hashtags: r.hashtags, avatar: r.avatar, status: r.status, videoJobId: r.video_job_id ?? undefined, videoUrl: r.video_url ?? undefined, error: r.error ?? undefined });

async function withAds(rows: ProjectRow[]): Promise<Project[]> {
  if (!rows.length) return [];
  const ads = await query<AdRow & { project_id: string }>("select * from ads where project_id = any($1) order by position", [rows.map((r) => r.id)]);
  return rows.map((r) => ({ id: r.id, userId: r.user_id, url: r.url, product: r.product, createdAt: r.created_at.toISOString(), ads: ads.filter((a) => a.project_id === r.id).map(adFromRow) }));
}

export async function getProject(id: string, userId: string) {
  if (!/^[0-9a-f-]{36}$/i.test(id)) return null;
  return (await withAds(await query<ProjectRow>("select * from projects where id = $1 and user_id = $2", [id, userId])))[0] ?? null;
}

export async function listProjects(userId: string, limit = 50) {
  return withAds(await query<ProjectRow>("select * from projects where user_id = $1 order by created_at desc limit $2", [userId, limit]));
}

export const balanceOf = async (userId: string) => Number((await query<{ b: string }>("select coalesce(sum(delta), 0) b from credit_ledger where user_id = $1", [userId]))[0].b);
