"use client";
import { useState } from "react";
import Link from "next/link";
import Logo from "@/components/Logo";

export default function AuthForm({ mode }: { mode: "login" | "signup" }) {
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true); setError("");
    const f = new FormData(e.currentTarget);
    const r = await fetch(`/api/auth/${mode}`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ email: f.get("email"), password: f.get("password") }) });
    const j = await r.json();
    if (r.ok && j.confirm) { setNotice("Check your email to confirm your account, then log in."); setBusy(false); return; }
    if (r.ok) { window.location.href = "/dashboard"; return; }
    setError(j.error); setBusy(false);
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center px-6">
      <div className="mb-8"><Logo /></div>
      <h1 className="mb-6 text-2xl font-semibold">{mode === "login" ? "Welcome back" : "Create your account"}</h1>
      <form onSubmit={submit} className="space-y-3">
        <input name="email" type="email" required placeholder="you@store.com" className="w-full rounded-lg border border-white/15 bg-white/5 px-4 py-3 outline-none focus:border-fuchsia-400" />
        <input name="password" type="password" required minLength={8} placeholder="Password (8+ characters)" className="w-full rounded-lg border border-white/15 bg-white/5 px-4 py-3 outline-none focus:border-fuchsia-400" />
        {error && <p className="text-sm text-red-400">{error}</p>}
        {notice && <p className="text-sm text-emerald-300">{notice}</p>}
        <button disabled={busy} className="w-full rounded-lg bg-fuchsia-500 py-3 font-semibold hover:bg-fuchsia-400 disabled:opacity-50">{busy ? "..." : mode === "login" ? "Log in" : "Sign up free"}</button>
      </form>
      <p className="mt-6 text-sm text-white/60">
        {mode === "login" ? <>No account? <Link className="text-fuchsia-300" href="/signup">Sign up</Link></> : <>Have an account? <Link className="text-fuchsia-300" href="/login">Log in</Link></>}
      </p>
    </main>
  );
}
