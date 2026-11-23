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
