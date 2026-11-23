import Link from "next/link";
import { PLANS } from "@/lib/plans";

const steps = [
  ["Paste your product link", "Shopify, WooCommerce or any product page. We pull the title, price, images and description."],
  ["We write the ads", "Scroll-stopping hooks, spoken scripts and captions with hashtags, each with a different angle."],
  ["Get UGC videos", "A realistic AI creator presents your product in vertical 9:16 video, ready for TikTok and Reels."],
];

export default function Home() {
  return (
    <main>
      <nav className="mx-auto flex max-w-5xl items-center justify-between px-6 py-5">
        <span className="text-xl font-bold">AdReel</span>
        <div className="flex gap-4 text-sm"><Link href="/login" className="text-white/70 hover:text-white">Log in</Link><Link href="/signup" className="rounded-lg bg-fuchsia-500 px-4 py-2 font-semibold hover:bg-fuchsia-400">Start free</Link></div>
      </nav>
      <section className="mx-auto max-w-3xl px-6 py-24 text-center">
        <h1 className="text-5xl font-extrabold leading-tight">UGC video ads for your store, <span className="text-fuchsia-400">from a product link</span></h1>
        <p className="mt-6 text-lg text-white/70">Skip creators, shipping samples and weeks of waiting. Paste a URL and get TikTok and Reels ads with AI avatars, hooks and captions in minutes.</p>
        <Link href="/signup" className="mt-10 inline-block rounded-xl bg-fuchsia-500 px-8 py-4 text-lg font-semibold hover:bg-fuchsia-400">Generate your first ad free</Link>
      </section>
      <section className="mx-auto grid max-w-5xl gap-4 px-6 pb-20 md:grid-cols-3">
        {steps.map(([t, d], i) => <div key={t} className="rounded-2xl border border-white/10 bg-white/5 p-6"><div className="mb-2 text-fuchsia-400">0{i + 1}</div><div className="mb-1 font-semibold">{t}</div><div className="text-sm text-white/60">{d}</div></div>)}
      </section>
      <section className="mx-auto max-w-5xl px-6 pb-24">
        <h2 className="mb-8 text-center text-3xl font-bold">Simple pricing</h2>
        <div className="grid gap-4 md:grid-cols-3">
          {(["starter", "growth", "scale"] as const).map((id) => (
            <div key={id} className="rounded-2xl border border-white/10 bg-white/5 p-6">
              <div className="font-semibold">{PLANS[id].name}</div>
              <div className="my-2 text-4xl font-bold">${PLANS[id].price}<span className="text-base font-normal text-white/50">/mo</span></div>
              <div className="text-sm text-white/70">{PLANS[id].videos} video ads per month</div>
              <div className="mb-5 text-sm text-white/50">{PLANS[id].blurb}</div>
              <Link href="/signup" className="block rounded-lg bg-white/10 py-2 text-center text-sm hover:bg-white/20">Get started</Link>
            </div>
          ))}
        </div>
      </section>
    </main>
  );
}
