"use client";
import { useEffect, useState } from "react";
import type { Project } from "@/lib/db";

export default function ProjectView({ id, initial }: { id: string; initial: Project }) {
  const [p, setP] = useState(initial);
  const active = p.ads.some((a) => a.status === "processing");

  useEffect(() => {
    if (!active) return;
    const t = setInterval(async () => { const r = await fetch(`/api/projects/${id}`); if (r.ok) setP(await r.json()); }, 4000);
    return () => clearInterval(t);
  }, [active, id]);

  return (
    <div className="grid gap-6 md:grid-cols-3">
      {p.ads.map((a, i) => (
        <div key={i} className="flex flex-col gap-3 rounded-2xl border border-white/10 bg-white/5 p-4">
          <div className="aspect-[9/16] overflow-hidden rounded-xl bg-black">
            {a.status === "done" && a.videoUrl ? <video src={a.videoUrl} controls playsInline className="h-full w-full object-cover" />
              : a.status === "failed" ? <div className="flex h-full items-center justify-center p-4 text-center text-sm text-red-400">{a.error ?? "Generation failed"}</div>
              : <div className="flex h-full animate-pulse items-center justify-center text-sm text-white/60">Generating video...</div>}
          </div>
          {a.videoUrl && <a href={a.videoUrl} download target="_blank" className="rounded-lg bg-fuchsia-500 py-2 text-center text-sm font-semibold hover:bg-fuchsia-400">Download</a>}
          <div><div className="text-xs uppercase text-white/40">Hook</div><div className="font-semibold">{a.hook}</div></div>
          <div><div className="text-xs uppercase text-white/40">Script</div><div className="text-sm text-white/80">{a.script}</div></div>
          <div>
            <div className="text-xs uppercase text-white/40">Caption</div>
            <div className="text-sm text-white/80">{a.caption} <span className="text-fuchsia-300">{a.hashtags.join(" ")}</span></div>
            <button className="mt-1 text-xs text-fuchsia-300" onClick={() => navigator.clipboard.writeText(`${a.caption} ${a.hashtags.join(" ")}`)}>Copy caption</button>
          </div>
        </div>
      ))}
    </div>
  );
}
