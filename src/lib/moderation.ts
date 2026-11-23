import type { Product } from "./db";

const BLOCKED: [RegExp, string][] = [
  [/\b(firearm|rifle|pistol|ammunition|ammo|silencer|suppressor|switchblade)s?\b/i, "weapons"],
  [/\b(porn|xxx|escort|erotic|adult toy|sex toy|fetish)\b/i, "adult content"],
  [/\b(cocaine|heroin|meth(amphetamine)?|lsd|mdma|cannabis|marijuana|weed|thc|cbd|kratom)\b/i, "drugs"],
  [/\b(casino|sports ?betting|poker|slots|gambling)\b/i, "gambling"],
  [/\b(cigarette|vape|vaping|e-?liquid|tobacco|nicotine)\b/i, "tobacco and vaping"],
  [/\b(counterfeit|replica|knock-?off|fake id|fake passport)\b/i, "counterfeit goods"],
  [/\b(payday loan|get rich quick|crypto signals|forex signals|ponzi)\b/i, "deceptive financial offers"],
  [/\b(miracle cure|cures? (cancer|diabetes|covid|aids)|lose \d+ ?(lbs|pounds|kg) in)\b/i, "unsubstantiated health claims"],
];

const BLOCKED_CLAIMS = /\b(cures?|guaranteed (results|income|weight loss)|clinically proven|doctor recommended|fda approved|no side effects)\b/i;

export function checkProduct(p: Product): string | null {
  const text = `${p.title} ${p.description}`;
  for (const [re, label] of BLOCKED) if (re.test(text)) return `We can't create ads for this product category (${label}).`;
  return null;
}

export function scrubClaims<T extends { hook: string; script: string; caption: string }>(c: T): T {
  const clean = (s: string) => s.replace(new RegExp(BLOCKED_CLAIMS.source, "gi"), "").replace(/\s{2,}/g, " ").trim();
  return { ...c, hook: clean(c.hook), script: clean(c.script), caption: clean(c.caption) };
}

export async function checkWithOpenAI(p: Product): Promise<string | null> {
  if (!process.env.OPENAI_API_KEY) return null;
  try {
    const r = await fetch("https://api.openai.com/v1/moderations", { method: "POST", headers: { authorization: `Bearer ${process.env.OPENAI_API_KEY}`, "content-type": "application/json" }, body: JSON.stringify({ model: "omni-moderation-latest", input: `${p.title}\n${p.description}`.slice(0, 4000) }), signal: AbortSignal.timeout(8000) });
    if (!r.ok) return null;
    return (await r.json()).results?.[0]?.flagged ? "This product page was flagged by content moderation." : null;
  } catch { return null; }
}
