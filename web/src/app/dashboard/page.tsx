import { Suspense } from "react";
import Link from "next/link";
import { getMe, listProjects } from "@/lib/api";
import { PLANS } from "@/lib/plans";
import NewProject from "@/components/NewProject";
import PlanPicker from "@/components/PlanPicker";
import LogoutButton from "@/components/LogoutButton";
import Logo from "@/components/Logo";

export default function Page() {
  return <Suspense fallback={<div className="p-10 text-white/50">Loading...</div>}><Dashboard /></Suspense>;
}

async function Dashboard() {
  const [user, projects] = await Promise.all([getMe(), listProjects()]);
  const remaining = Math.max(0, user.balance);

  return (
    <main className="mx-auto max-w-4xl space-y-8 px-6 py-10">
      <header className="flex items-center justify-between">
        <Logo />
        <div className="flex items-center gap-4 text-sm text-white/70"><span>{user.email}</span><LogoutButton /></div>
      </header>
      {user.mock && <div className="rounded-lg border border-amber-400/30 bg-amber-400/10 p-3 text-sm text-amber-200">Demo mode: no video provider is configured on the API, so videos are placeholder clips.</div>}
      <section className="flex items-center justify-between rounded-2xl border border-white/10 bg-white/5 p-5">
        <div><div className="text-sm text-white/50">{PLANS[user.plan].name}</div><div className="text-2xl font-bold">{remaining} <span className="text-base font-normal text-white/50">videos left</span></div></div>
      </section>
      <NewProject remaining={remaining} />
      <section>
        <h2 className="mb-3 text-lg font-semibold">Your projects</h2>
        {projects.length === 0 ? <div className="rounded-2xl border border-dashed border-white/15 p-8 text-center text-white/50">No projects yet. Paste a product link above to create your first ads.</div> : (
          <ul className="space-y-2">
            {projects.map((p) => (
              <li key={p.id}><Link href={`/projects/${p.id}`} className="flex items-center gap-4 rounded-xl border border-white/10 bg-white/5 p-3 hover:bg-white/10">
                {p.product.image && <img src={p.product.image} alt="" className="h-12 w-12 rounded-lg object-cover" />}
                <div className="min-w-0 flex-1"><div className="truncate font-medium">{p.product.title}</div><div className="text-xs text-white/50">{p.ads.length} video{p.ads.length > 1 ? "s" : ""} - {new Date(p.createdAt).toLocaleDateString()} - {p.ads.every((a) => a.status === "done") ? "Ready" : p.ads.some((a) => a.status === "pending" || a.status === "processing") ? "Generating" : "Some failed"}</div></div>
              </Link></li>
            ))}
          </ul>
        )}
      </section>
      <section><h2 className="mb-3 text-lg font-semibold">Upgrade</h2><PlanPicker current={user.plan} hasBilling={user.hasBilling} /></section>
    </main>
  );
}
