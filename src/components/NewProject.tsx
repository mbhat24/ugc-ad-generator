"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";

export default function NewProject({ remaining }: { remaining: number }) {
  const router = useRouter();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const max = Math.min(3, remaining);

  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true); setError("");
    const f = new FormData(e.currentTarget);
    const r = await fetch("/api/projects", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ url: f.get("url"), count: Number(f.get("count")) }) });
    const j = await r.json();
    if (r.ok) router.push(`/projects/${j.id}`);
    else { setError(j.error); setBusy(false); }
  }

  return (
    <form onSubmit={submit} className="space-y-3 rounded-2xl border border-white/10 bg-white/5 p-5">
      <label className="text-sm font-medium">Product URL</label>
      <div className="flex flex-col gap-3 sm:flex-row">
        <input name="url" type="url" required placeholder="https://yourstore.com/products/..." className="flex-1 rounded-lg border border-white/15 bg-black/30 px-4 py-3 outline-none focus:border-fuchsia-400" />
        <select name="count" defaultValue={max} disabled={max < 1} className="rounded-lg border border-white/15 bg-black/30 px-3 py-3">
          {Array.from({ length: Math.max(max, 1) }, (_, i) => <option key={i} value={i + 1}>{i + 1} video{i ? "s" : ""}</option>)}
        </select>
        <button disabled={busy || max < 1} className="rounded-lg bg-fuchsia-500 px-6 py-3 font-semibold hover:bg-fuchsia-400 disabled:opacity-50">{busy ? "Generating..." : "Generate ads"}</button>
      </div>
      {error && <p className="text-sm text-red-400">{error}</p>}
      {max < 1 && <p className="text-sm text-amber-300">You are out of videos this month. Upgrade below.</p>}
    </form>
  );
}
