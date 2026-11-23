import { createHash } from "crypto";
import { cookies } from "next/headers";

export const devAuth = !process.env.NEXT_PUBLIC_SUPABASE_URL && process.env.NODE_ENV !== "production";

const COOKIE = "dev_user";
export const devId = (email: string) => {
  const h = createHash("sha256").update(email.toLowerCase()).digest("hex");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-4${h.slice(13, 16)}-8${h.slice(17, 20)}-${h.slice(20, 32)}`;
};

export async function devLogin(email: string) {
  (await cookies()).set(COOKIE, encodeURIComponent(email.toLowerCase()), { httpOnly: true, sameSite: "lax", path: "/", maxAge: 86400 * 30 });
}
export async function devLogout() { (await cookies()).delete(COOKIE); }
export async function devEmail() {
  const v = (await cookies()).get(COOKIE)?.value;
  return v ? decodeURIComponent(v) : null;
}
