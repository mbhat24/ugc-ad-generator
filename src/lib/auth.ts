import { randomBytes, randomUUID, scryptSync, timingSafeEqual } from "crypto";
import { cookies } from "next/headers";
import { withDB, type User } from "./db";
import { PLANS } from "./plans";

const COOKIE = "session";

export const hashPassword = (pw: string) => {
  const salt = randomBytes(16).toString("hex");
  return `${salt}:${scryptSync(pw, salt, 64).toString("hex")}`;
};

const verifyPassword = (pw: string, stored: string) => {
  const [salt, hash] = stored.split(":");
  return timingSafeEqual(Buffer.from(hash, "hex"), scryptSync(pw, salt, 64));
};

export async function createSession(userId: string) {
  const token = randomBytes(32).toString("hex");
  await withDB((db) => void db.sessions.push({ token, userId }));
  (await cookies()).set(COOKIE, token, { httpOnly: true, sameSite: "lax", secure: process.env.NODE_ENV === "production", path: "/", maxAge: 60 * 60 * 24 * 30 });
}

export async function destroySession() {
  const jar = await cookies();
  const token = jar.get(COOKIE)?.value;
  if (token) await withDB((db) => void (db.sessions = db.sessions.filter((s) => s.token !== token)));
  jar.delete(COOKIE);
}

export async function currentUser(): Promise<User | null> {
  const token = (await cookies()).get(COOKIE)?.value;
  if (!token) return null;
  return withDB((db) => {
    const s = db.sessions.find((x) => x.token === token);
    const u = s && db.users.find((x) => x.id === s.userId);
    if (!u) return null;
    const now = new Date();
    if (now.getTime() - new Date(u.periodStart).getTime() > 30 * 864e5) { u.usedThisMonth = 0; u.periodStart = now.toISOString(); }
    return u;
  });
}

export async function signup(email: string, password: string) {
  email = email.trim().toLowerCase();
  if (!/^\S+@\S+\.\S+$/.test(email) || password.length < 8) throw new Error("Enter a valid email and a password of 8+ characters");
  const user = await withDB((db) => {
    if (db.users.some((u) => u.email === email)) throw new Error("Email already registered");
    const u: User = { id: randomUUID(), email, passwordHash: hashPassword(password), plan: "free", usedThisMonth: 0, periodStart: new Date().toISOString() };
    db.users.push(u);
    return u;
  });
  await createSession(user.id);
}

export async function login(email: string, password: string) {
  const user = await withDB((db) => db.users.find((u) => u.email === email.trim().toLowerCase()));
  if (!user || !verifyPassword(password, user.passwordHash)) throw new Error("Invalid email or password");
  await createSession(user.id);
}

export const quotaOf = (u: User) => PLANS[u.plan].videos;
