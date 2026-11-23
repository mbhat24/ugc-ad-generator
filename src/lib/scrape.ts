import type { Project } from "./db";

const decode = (s: string) => s.replace(/&amp;/g, "&").replace(/&quot;/g, '"').replace(/&#0?39;/g, "'").replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();
const meta = (html: string, key: string) => {
  const m = html.match(new RegExp(`<meta[^>]+(?:property|name)=["']${key}["'][^>]*content=["']([^"']*)["']`, "i")) ?? html.match(new RegExp(`<meta[^>]+content=["']([^"']*)["'][^>]*(?:property|name)=["']${key}["']`, "i"));
  return m ? decode(m[1]) : undefined;
};

function assertPublicUrl(raw: string) {
  const u = new URL(raw);
  if (!/^https?:$/.test(u.protocol)) throw new Error("URL must be http(s)");
  const h = u.hostname;
  if (h === "localhost" || /^(127\.|10\.|192\.168\.|169\.254\.|172\.(1[6-9]|2\d|3[01])\.|0\.)/.test(h) || h.includes(":") || h.endsWith(".local") || h.endsWith(".internal")) throw new Error("URL must be a public website");
  return u;
}

export async function scrapeProduct(rawUrl: string): Promise<Project["product"]> {
  const u = assertPublicUrl(rawUrl);
  const headers = { "user-agent": "Mozilla/5.0 (compatible; UGCAdBot/1.0)" };
  const opts = { headers, signal: AbortSignal.timeout(12000), redirect: "follow" as const };

  const jsonUrl = u.pathname.includes("/products/") ? `${u.origin}${u.pathname.replace(/\/$/, "")}.json` : null;
  if (jsonUrl) {
    try {
      const r = await fetch(jsonUrl, opts);
      if (r.ok) {
        const p = (await r.json()).product;
        const images: string[] = (p.images ?? []).map((i: { src: string }) => i.src);
        return { title: p.title, description: decode(p.body_html ?? ""), price: p.variants?.[0]?.price ? `$${p.variants[0].price}` : undefined, image: images[0], images: images.slice(0, 5) };
      }
    } catch {}
  }

  const r = await fetch(u, opts);
  if (!r.ok) throw new Error(`Could not load that page (HTTP ${r.status})`);
  const html = (await r.text()).slice(0, 1_500_000);
  const title = meta(html, "og:title") ?? decode(html.match(/<title[^>]*>([\s\S]*?)<\/title>/i)?.[1] ?? "");
  if (!title) throw new Error("Could not find a product on that page");
  const image = meta(html, "og:image");
  const price = meta(html, "product:price:amount") ?? meta(html, "og:price:amount");
  const cur = meta(html, "product:price:currency") ?? meta(html, "og:price:currency");
  return { title, description: meta(html, "og:description") ?? meta(html, "description") ?? "", price: price ? `${cur === "USD" || !cur ? "$" : cur + " "}${price}` : undefined, image, images: image ? [image] : [] };
}
