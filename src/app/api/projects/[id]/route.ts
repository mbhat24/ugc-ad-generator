import { currentUser } from "@/lib/auth";
import { getProject } from "@/lib/db";
import { signProject } from "@/lib/media";

export async function GET(_: Request, ctx: RouteContext<"/api/projects/[id]">) {
  const { id } = await ctx.params;
  const user = await currentUser();
  if (!user) return Response.json({ error: "Not signed in" }, { status: 401 });
  const project = await getProject(id, user.id);
  return project ? Response.json(await signProject(project)) : Response.json({ error: "Not found" }, { status: 404 });
}
