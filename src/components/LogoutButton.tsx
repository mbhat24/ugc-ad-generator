"use client";

export default function LogoutButton() {
  return <button className="text-sm text-white/60 hover:text-white" onClick={async () => { await fetch("/api/auth/logout", { method: "POST" }); window.location.href = "/"; }}>Log out</button>;
}
