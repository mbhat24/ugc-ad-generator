import { currentUser } from "@/lib/auth";
import { query, listProjects } from "@/lib/db";
import { fail } from "@/lib/http";
import { signProject } from "@/lib/media";

export async function GET() {
  const user = await currentUser();
  if (!user) return fail("Not signed in", 401);
  const [profile] = await query("select id, email, plan, created_at from profiles where id = $1", [user.id]);
  const ledger = await query("select delta, reason, ref, created_at from credit_ledger where user_id = $1 order by id", [user.id]);
  const projects = await Promise.all((await listProjects(user.id, 1000)).map(signProject));
  return new Response(JSON.stringify({ profile, projects, ledger }, null, 2), { headers: { "content-type": "application/json", "content-disposition": 'attachment; filename="adreel-export.json"' } });
}
