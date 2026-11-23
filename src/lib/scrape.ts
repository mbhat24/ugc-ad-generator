import { lookup } from "dns/promises";
import { isIP } from "net";
import type { Project } from "./db";

const decode = (s: string) => s.replace(/&amp;/g, "&").replace(/&quot;/g, '"').replace(/&#0?39;/g, "'").replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();
const meta = (html: string, key: string) => {
  const m = html.match(new RegExp(`<meta[^>]+(?:property|name)=["']${key}["'][^>]*content=["']([^"']*)["']`, "i")) ?? html.match(new RegExp(`<meta[^>]+content=["']([^"']*)["'][^>]*(?:property|name)=["']${key}["']`, "i"));
  return m ? decode(m[1]) : undefined;
};

const privateIp = (ip: string) => {
  if (isIP(ip) === 6) {
    const l = ip.toLowerCase();
    if (l.startsWith("::ffff:")) return privateIp(l.slice(7));
    return l === "::1" || l === "::" || /^f[cd]/.test(l) || /^fe[89ab]/.test(l);
  }
  const [a, b] = ip.split(".").map(Number);
  return a === 0 || a === 10 || a === 127 || (a === 169 && b === 254) || (a === 172 && b >= 16 && b <= 31) || (a === 192 && b === 168) || (a === 100 && b >= 64 && b <= 127) || a >= 224;
};

async function assertPublicUrl(raw: string) {
  const u = new URL(raw);
  if (!/^https?:$/.test(u.protocol)) throw new Error("URL must be http(s)");
  const h = u.hostname.replace(/^\[|\]$/g, "");
  const addrs = isIP(h) ? [{ address: h }] : await lookup(h, { all: true }).catch(() => []);
  if (!addrs.length || addrs.some((a) => privateIp(a.address))) throw new Error("URL must be a public website");
  return u;
}

async function safeFetch(raw: string, init: RequestInit) {
  let url = raw;
  for (let i = 0; i < 4; i++) {
    const u = await assertPublicUrl(url);
    const r = await fetch(u, { ...init, redirect: "manual" });
    const loc = r.headers.get("location");
    if (r.status >= 300 && r.status < 400 && loc) { url = new URL(loc, u).toString(); continue; }
    return r;
  }
  throw new Error("Too many redirects");
}

export async function scrapeProduct(rawUrl: string): Promise<Project["product"]> {
  const u = await assertPublicUrl(rawUrl);
  const headers = { "user-agent": "Mozilla/5.0 (compatible; UGCAdBot/1.0)" };
  const opts = { headers, signal: AbortSignal.timeout(12000) };

  const jsonUrl = u.pathname.includes("/products/") ? `${u.origin}${u.pathname.replace(/\/$/, "")}.json` : null;
  if (jsonUrl) {
    try {
      const r = await safeFetch(jsonUrl, opts);
      if (r.ok) {
        const p = (await r.json()).product;
        const images: string[] = (p.images ?? []).map((i: { src: string }) => i.src);
        return { title: p.title, description: decode(p.body_html ?? ""), price: p.variants?.[0]?.price ? `$${p.variants[0].price}` : undefined, image: images[0], images: images.slice(0, 5) };
      }
    } catch {}
  }

  const r = await safeFetch(u.toString(), opts);
  if (!r.ok) throw new Error(`Could not load that page (HTTP ${r.status})`);
  const html = (await r.text()).slice(0, 1_500_000);
  const title = meta(html, "og:title") ?? decode(html.match(/<title[^>]*>([\s\S]*?)<\/title>/i)?.[1] ?? "");
  if (!title) throw new Error("Could not find a product on that page");
  const image = meta(html, "og:image");
  const price = meta(html, "product:price:amount") ?? meta(html, "og:price:amount");
  const cur = meta(html, "product:price:currency") ?? meta(html, "og:price:currency");
  return { title, description: meta(html, "og:description") ?? meta(html, "description") ?? "", price: price ? `${cur === "USD" || !cur ? "$" : cur + " "}${price}` : undefined, image, images: image ? [image] : [] };
}
