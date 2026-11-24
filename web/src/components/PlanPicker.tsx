"use client";
import { useState } from "react";
import { PLANS } from "@/lib/plans";
import type { PlanId } from "@/lib/types";

export default function PlanPicker({ current, hasBilling }: { current: PlanId; hasBilling: boolean }) {
  const [error, setError] = useState("");
  async function go(path: string, body?: object) {
    setError("");
    const r = await fetch(path, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body ?? {}) });
    const j = await r.json();
    if (r.ok && j.url) window.location.assign(j.url); else setError(j.error ?? "Something went wrong");
  }
  return (
    <div>
      <div className="grid gap-3 sm:grid-cols-3">
        {(["starter", "growth", "scale"] as const).map((id) => (
          <div key={id} className="rounded-xl border border-white/10 bg-white/5 p-4">
            <div className="font-semibold">{PLANS[id].name}</div>
            <div className="text-2xl font-bold">${PLANS[id].price}<span className="text-sm font-normal text-white/50">/mo</span></div>
            <div className="mb-3 text-sm text-white/60">{PLANS[id].videos} videos / month</div>
            <button disabled={current === id} onClick={() => go(current === "free" ? "/api/v1/billing/checkout" : "/api/v1/billing/portal", { plan: id })} className="w-full rounded-lg bg-white/10 py-2 text-sm hover:bg-white/20 disabled:opacity-40">{current === id ? "Current plan" : current === "free" ? "Choose" : "Change in portal"}</button>
          </div>
        ))}
      </div>
      {hasBilling && <button onClick={() => go("/api/v1/billing/portal")} className="mt-3 text-sm text-fuchsia-300">Manage billing</button>}
      {error && <p className="mt-2 text-sm text-amber-300">{error}</p>}
    </div>
  );
}
