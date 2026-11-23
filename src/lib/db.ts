import { promises as fs } from "fs";
import path from "path";
import type { PlanId } from "./plans";

export type User = { id: string; email: string; passwordHash: string; plan: PlanId; usedThisMonth: number; periodStart: string };
export type Session = { token: string; userId: string };
export type Ad = { hook: string; script: string; caption: string; hashtags: string[]; avatar: string; videoJobId?: string; status: "pending" | "processing" | "done" | "failed"; videoUrl?: string; error?: string };
export type Project = { id: string; userId: string; url: string; product: { title: string; description: string; price?: string; image?: string; images: string[] }; ads: Ad[]; createdAt: string };
type DB = { users: User[]; sessions: Session[]; projects: Project[] };

const FILE = path.join(process.cwd(), "data", "db.json");
let lock: Promise<unknown> = Promise.resolve();

async function read(): Promise<DB> {
  try { return JSON.parse(await fs.readFile(FILE, "utf8")); } catch { return { users: [], sessions: [], projects: [] }; }
}

export function withDB<T>(fn: (db: DB) => T | Promise<T>): Promise<T> {
  const run = lock.then(async () => {
    const db = await read();
    const result = await fn(db);
    await fs.mkdir(path.dirname(FILE), { recursive: true });
    await fs.writeFile(FILE, JSON.stringify(db, null, 2));
    return result;
  });
  lock = run.catch(() => {});
  return run;
}
