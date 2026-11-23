import { currentUser } from "@/lib/auth";
import { withDB } from "@/lib/db";
import { provider } from "@/lib/video";

export async function GET(_: Request, ctx: RouteContext<"/api/projects/[id]">) {
  const { id } = await ctx.params;
  const user = await currentUser();
  if (!user) return Response.json({ error: "Not signed in" }, { status: 401 });
  const project = await withDB((db) => db.projects.find((p) => p.id === id && p.userId === user.id));
  if (!project) return Response.json({ error: "Not found" }, { status: 404 });

  const pending = project.ads.filter((a) => a.status === "processing" && a.videoJobId);
  if (pending.length) {
    const states = await Promise.all(pending.map((a) => provider.poll(a.videoJobId!).catch(() => ({ status: "processing" as const }))));
    return Response.json(await withDB((db) => {
      const p = db.projects.find((x) => x.id === id)!;
      pending.forEach((a, i) => {
        const ad = p.ads.find((x) => x.videoJobId === a.videoJobId)!;
        const s = states[i];
        if (s.status === "processing" || ad.status !== "processing") return;
        ad.status = s.status; ad.videoUrl = s.videoUrl; ad.error = s.error;
        if (s.status === "failed") { const u = db.users.find((x) => x.id === user.id)!; u.usedThisMonth = Math.max(0, u.usedThisMonth - 1); }
      });
      return p;
    }));
  }
  return Response.json(project);
}
