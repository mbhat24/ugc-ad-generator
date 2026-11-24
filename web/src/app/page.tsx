import Link from "next/link";
import { PLANS } from "@/lib/plans";
import Logo from "@/components/Logo";

const steps = [
  ["Paste your product link", "Shopify, WooCommerce or any product page. We pull the title, price, images and description."],
  ["We write the ads", "Scroll-stopping hooks, spoken scripts and captions with hashtags, each with a different angle."],
  ["Get UGC videos", "A realistic AI creator presents your product in vertical 9:16 video, ready for TikTok and Reels."],
];

const features = [
  ["Hooks that stop the scroll", "Every variant tests a different angle so you find winners faster."],
  ["Captions burned in", "Readable on mute, exported with the video, ready to upload."],
  ["9:16 vertical, ad-ready", "Made for TikTok, Instagram Reels and YouTube Shorts."],
  ["Pay per video, not per creator", "No shipping samples, no scheduling, no revisions."],
];

const faqs = [
  ["How long does a video take?", "Usually a few minutes. You can leave the page and come back; your ads keep generating."],
  ["Do I need a Shopify store?", "No. Any public product page works. Shopify product links give the richest data."],
  ["What if a video fails?", "The credit is refunded automatically."],
  ["Can I use the videos in paid ads?", "Yes. Follow each platform's rules on labeling AI-generated content."],
];

const hooks = ["Stop scrolling, this is real", "I was skeptical until day 3", "Why everyone is switching"];

export default function Home() {
  return (
    <main>
      <nav className="mx-auto flex max-w-6xl items-center justify-between px-6 py-5">
        <Logo />
        <div className="flex items-center gap-5 text-sm">
          <a href="#pricing" className="hidden text-white/70 hover:text-white sm:block">Pricing</a>
          <a href="#faq" className="hidden text-white/70 hover:text-white sm:block">FAQ</a>
          <Link href="/login" className="text-white/70 hover:text-white">Log in</Link>
          <Link href="/signup" className="rounded-lg bg-fuchsia-500 px-4 py-2 font-semibold hover:bg-fuchsia-400">Start free</Link>
        </div>
      </nav>

      <section className="relative mx-auto grid max-w-6xl items-center gap-12 px-6 py-16 md:grid-cols-2 md:py-24">
        <div className="pointer-events-none absolute -top-10 left-1/2 h-72 w-72 -translate-x-1/2 rounded-full bg-fuchsia-600/20 blur-3xl" />
        <div className="relative">
          <span className="rounded-full border border-fuchsia-400/30 bg-fuchsia-400/10 px-3 py-1 text-xs text-fuchsia-200">AI UGC video ads</span>
          <h1 className="mt-5 text-4xl font-extrabold leading-tight sm:text-5xl">UGC video ads for your store, <span className="bg-gradient-to-r from-fuchsia-400 to-violet-400 bg-clip-text text-transparent">from a product link</span></h1>
          <p className="mt-5 text-lg text-white/70">Skip creators, shipping samples and weeks of waiting. Paste a URL and get TikTok and Reels ads with AI avatars, hooks and captions in minutes.</p>
          <div className="mt-8 flex flex-wrap items-center gap-4">
            <Link href="/signup" className="rounded-xl bg-fuchsia-500 px-7 py-3.5 text-lg font-semibold hover:bg-fuchsia-400">Generate your first ad free</Link>
            <a href="#how" className="text-sm text-white/70 hover:text-white">See how it works &rarr;</a>
          </div>
          <p className="mt-4 text-xs text-white/40">No credit card. One free video to try it.</p>
        </div>
        <div className="relative mx-auto flex gap-4">
          {hooks.map((h, i) => (
            <div key={h} className={`w-36 rounded-2xl border border-white/10 bg-gradient-to-b from-white/10 to-white/[0.02] p-2 sm:w-40 ${i === 1 ? "-translate-y-6" : i === 2 ? "translate-y-4" : ""}`}>
              <div className="relative aspect-[9/16] overflow-hidden rounded-xl bg-gradient-to-br from-fuchsia-700/60 via-violet-800/50 to-slate-900">
                <div className="absolute inset-x-2 bottom-8 rounded bg-black/60 px-2 py-1 text-center text-[11px] font-bold leading-tight">{h}</div>
                <div className="absolute right-2 top-2 rounded-full bg-black/50 px-1.5 text-[10px]">9:16</div>
              </div>
            </div>
          ))}
        </div>
      </section>

      <section id="how" className="mx-auto grid max-w-6xl gap-4 px-6 pb-20 md:grid-cols-3">
        {steps.map(([t, d], i) => <div key={t} className="rounded-2xl border border-white/10 bg-white/5 p-6"><div className="mb-2 text-fuchsia-400">0{i + 1}</div><div className="mb-1 font-semibold">{t}</div><div className="text-sm text-white/60">{d}</div></div>)}
      </section>

      <section className="mx-auto max-w-6xl px-6 pb-20">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {features.map(([t, d]) => <div key={t} className="rounded-2xl border border-white/10 p-5"><div className="mb-1 font-semibold">{t}</div><div className="text-sm text-white/60">{d}</div></div>)}
        </div>
      </section>

      <section id="pricing" className="mx-auto max-w-6xl px-6 pb-20">
        <h2 className="mb-2 text-center text-3xl font-bold">Simple pricing</h2>
        <p className="mb-8 text-center text-white/60">Cancel anytime. Failed videos are refunded.</p>
        <div className="grid gap-4 md:grid-cols-3">
          {(["starter", "growth", "scale"] as const).map((id) => (
            <div key={id} className={`rounded-2xl border p-6 ${id === "growth" ? "border-fuchsia-400/50 bg-fuchsia-400/5" : "border-white/10 bg-white/5"}`}>
              <div className="flex items-center justify-between font-semibold">{PLANS[id].name}{id === "growth" && <span className="rounded-full bg-fuchsia-500 px-2 py-0.5 text-xs">Popular</span>}</div>
              <div className="my-2 text-4xl font-bold">${PLANS[id].price}<span className="text-base font-normal text-white/50">/mo</span></div>
              <div className="text-sm text-white/70">{PLANS[id].videos} video ads per month</div>
              <div className="mb-5 text-sm text-white/50">{PLANS[id].blurb}</div>
              <Link href="/signup" className="block rounded-lg bg-white/10 py-2 text-center text-sm hover:bg-white/20">Get started</Link>
            </div>
          ))}
        </div>
      </section>

      <section id="faq" className="mx-auto max-w-3xl px-6 pb-24">
        <h2 className="mb-6 text-center text-3xl font-bold">Questions</h2>
        <div className="space-y-3">
          {faqs.map(([q, a]) => <details key={q} className="group rounded-xl border border-white/10 bg-white/5 p-4"><summary className="cursor-pointer font-medium">{q}</summary><p className="mt-2 text-sm text-white/60">{a}</p></details>)}
        </div>
      </section>

      <footer className="border-t border-white/10 py-8 text-center text-xs text-white/40">&copy; AdReel. Videos are AI-generated; label them where platforms require.<div className="mt-2 space-x-4"><Link href="/terms" className="hover:text-white">Terms</Link><Link href="/privacy" className="hover:text-white">Privacy</Link><Link href="/acceptable-use" className="hover:text-white">Acceptable use</Link></div></footer>
    </main>
  );
}
