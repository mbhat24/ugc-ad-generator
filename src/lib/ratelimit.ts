import { query } from "./db";

export async function overUsage(userId: string) {
  const [r] = await query<{ hourly: string; daily: string }>(
    `select count(*) filter (where p.created_at > now() - interval '1 hour') hourly,
            count(a.id) filter (where p.created_at > now() - interval '1 day') daily
     from projects p left join ads a on a.project_id = p.id where p.user_id = $1`, [userId]);
  if (Number(r.hourly) >= Number(process.env.MAX_PROJECTS_PER_HOUR ?? 10)) return "Too many projects this hour. Try again later.";
  if (Number(r.daily) >= Number(process.env.MAX_ADS_PER_DAY ?? 30)) return "Daily generation limit reached. Try again tomorrow.";
  return null;
}

export function clientIp(req: Request) {
  const hops = Number(process.env.TRUSTED_PROXY_HOPS ?? 1);
  const parts = (req.headers.get("x-forwarded-for") ?? "").split(",").map((s) => s.trim()).filter(Boolean);
  return parts[parts.length - hops] ?? req.headers.get("x-real-ip") ?? "unknown";
}

export async function hit(key: string, max: number, windowSec: number) {
  const [r] = await query<{ count: number }>(
    `insert into rate_limits (key, window_start, count)
     values ($1, to_timestamp(floor(extract(epoch from now()) / $2) * $2), 1)
     on conflict (key, window_start) do update set count = rate_limits.count + 1
     returning count`, [key, windowSec]);
  return r.count > max;
}

export async function limited(req: Request, name: string, max: number, windowSec: number, extra = "") {
  return hit(`${name}:${clientIp(req)}${extra ? `:${extra}` : ""}`, max, windowSec);
}
