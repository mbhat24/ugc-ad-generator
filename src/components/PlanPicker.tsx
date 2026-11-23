"use client";
import { useState } from "react";
import { PLANS, type PlanId } from "@/lib/plans";

export default function PlanPicker({ current }: { current: PlanId }) {
  const [error, setError] = useState("");
  async function choose(plan: string) {
    const r = await fetch("/api/plan", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ plan }) });
    if (r.ok) window.location.reload(); else setError((await r.json()).error);
  }
  return (
    <div>
      <div className="grid gap-3 sm:grid-cols-3">
        {(["starter", "growth", "scale"] as const).map((id) => (
          <div key={id} className="rounded-xl border border-white/10 bg-white/5 p-4">
            <div className="font-semibold">{PLANS[id].name}</div>
            <div className="text-2xl font-bold">${PLANS[id].price}<span className="text-sm font-normal text-white/50">/mo</span></div>
            <div className="mb-3 text-sm text-white/60">{PLANS[id].videos} videos / month</div>
            <button disabled={current === id} onClick={() => choose(id)} className="w-full rounded-lg bg-white/10 py-2 text-sm hover:bg-white/20 disabled:opacity-40">{current === id ? "Current plan" : "Choose"}</button>
          </div>
        ))}
      </div>
      {error && <p className="mt-2 text-sm text-amber-300">{error}</p>}
    </div>
  );
}
