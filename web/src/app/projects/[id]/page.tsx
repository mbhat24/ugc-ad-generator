import { Suspense } from "react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { getProject } from "@/lib/api";
import ProjectView from "@/components/ProjectView";

export default function Page({ params }: PageProps<"/projects/[id]">) {
  return <Suspense fallback={<div className="p-10 text-white/50">Loading...</div>}><ProjectPage params={params} /></Suspense>;
}

async function ProjectPage({ params }: Pick<PageProps<"/projects/[id]">, "params">) {
  const { id } = await params;
  const project = await getProject(id);
  if (!project) notFound();
  return (
    <main className="mx-auto max-w-5xl space-y-6 px-6 py-10">
      <Link href="/dashboard" className="text-sm text-white/60 hover:text-white">&larr; Dashboard</Link>
      <h1 className="text-2xl font-bold">{project.product.title}</h1>
      <ProjectView id={project.id} initial={project} />
    </main>
  );
}
