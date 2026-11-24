import Link from "next/link";
import Logo from "@/components/Logo";

export default function Legal({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <main className="mx-auto max-w-3xl px-6 py-10">
      <Logo />
      <h1 className="mt-8 text-3xl font-bold">{title}</h1>
      <p className="mt-2 rounded-lg border border-amber-400/30 bg-amber-400/10 p-3 text-xs text-amber-200">Template provided for convenience. Replace the bracketed placeholders and have a qualified lawyer review it before you take payments.</p>
      <div className="mt-6 space-y-4 text-sm leading-relaxed text-white/80 [&_h2]:mt-6 [&_h2]:text-lg [&_h2]:font-semibold [&_h2]:text-white [&_li]:ml-5 [&_li]:list-disc">{children}</div>
      <p className="mt-10 text-xs text-white/40"><Link href="/terms" className="hover:text-white">Terms</Link> · <Link href="/privacy" className="hover:text-white">Privacy</Link> · <Link href="/acceptable-use" className="hover:text-white">Acceptable use</Link></p>
    </main>
  );
}
