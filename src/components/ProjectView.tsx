"use client";
import { useEffect, useState } from "react";
import type { Project } from "@/lib/db";

function Copy({ text, label }: { text: string; label: string }) {
  const [done, setDone] = useState(false);
  return <button className="text-xs text-fuchsia-300 hover:text-fuchsia-200" onClick={() => { navigator.clipboard.writeText(text); setDone(true); setTimeout(() => setDone(false), 1500); }}>{done ? "Copied" : label}</button>;
}

export default function ProjectView({ id, initial }: { id: string; initial: Project }) {
  const [p, setP] = useState(initial);
  const active = p.ads.some((a) => a.status === "processing" || a.status === "pending");

  useEffect(() => {
    if (!active) return;
    const t = setInterval(async () => { const r = await fetch(`/api/projects/${id}`); if (r.ok) setP(await r.json()); }, 4000);
    return () => clearInterval(t);
  }, [active, id]);

  return (
    <div className="space-y-4">
      {active && <p className="text-sm text-white/60">Your videos are generating. This usually takes a few minutes and you can leave this page.</p>}
      <div className="grid gap-6 md:grid-cols-3">
        {p.ads.map((a, i) => (
          <div key={a.id} className="flex flex-col gap-3 rounded-2xl border border-white/10 bg-white/5 p-4">
            <div className="flex items-center justify-between text-xs text-white/50"><span>Variant {i + 1}</span><span className={a.status === "done" ? "text-emerald-300" : a.status === "failed" ? "text-red-300" : "text-amber-300"}>{a.status === "done" ? "Ready" : a.status === "failed" ? "Failed, credit refunded" : "Generating"}</span></div>
            <div className="aspect-[9/16] overflow-hidden rounded-xl bg-black">
              {a.status === "done" && a.videoUrl ? <video src={a.videoUrl} controls playsInline className="h-full w-full object-cover" />
                : a.status === "failed" ? <div className="flex h-full items-center justify-center p-4 text-center text-sm text-red-400">{a.error ?? "Generation failed"}</div>
                : <div className="flex h-full animate-pulse flex-col items-center justify-center gap-2 text-sm text-white/60"><div className="h-8 w-8 animate-spin rounded-full border-2 border-white/20 border-t-fuchsia-400" />Generating video...</div>}
            </div>
            {a.videoUrl && <a href={a.videoUrl} download target="_blank" rel="noreferrer" className="rounded-lg bg-fuchsia-500 py-2 text-center text-sm font-semibold hover:bg-fuchsia-400">Download</a>}
            <div><div className="flex justify-between"><span className="text-xs uppercase text-white/40">Hook</span><Copy text={a.hook} label="Copy" /></div><div className="font-semibold">{a.hook}</div></div>
            <div><div className="flex justify-between"><span className="text-xs uppercase text-white/40">Script</span><Copy text={a.script} label="Copy" /></div><div className="text-sm text-white/80">{a.script}</div></div>
            <div><div className="flex justify-between"><span className="text-xs uppercase text-white/40">Caption</span><Copy text={`${a.caption} ${a.hashtags.join(" ")}`} label="Copy" /></div><div className="text-sm text-white/80">{a.caption} <span className="text-fuchsia-300">{a.hashtags.join(" ")}</span></div></div>
          </div>
        ))}
      </div>
    </div>
  );
}
