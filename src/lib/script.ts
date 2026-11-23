import type { Ad, Project } from "./db";

export const AVATARS = [
  "young woman, casual home setting, friendly and energetic",
  "young man, bright bedroom, relaxed and authentic",
  "woman in her 30s, kitchen, warm and trustworthy",
  "man in his 30s, living room, confident and conversational",
];

type Copy = { hook: string; script: string; caption: string; hashtags: string[] };

const templates = (p: Project["product"]): Copy[] => [
  { hook: `Stop scrolling — I can't believe this ${p.title} is real`, script: `Okay so I just tried the ${p.title} and honestly? I was skeptical. ${p.description.slice(0, 120)} Two weeks in and I use it every day. If you've been on the fence, this is your sign.`, caption: `Not sponsored, just obsessed with the ${p.title}. Link in bio.`, hashtags: ["#tiktokmademebuyit", "#musthave", "#review"] },
  { hook: `POV: you finally found the ${p.title} everyone keeps talking about`, script: `Everyone kept telling me about this, so I finally got it. ${p.description.slice(0, 120)} ${p.price ? `And it's only ${p.price}.` : ""} Go grab one before it sells out.`, caption: `Found it. ${p.title}, link in bio.`, hashtags: ["#fyp", "#smallbusiness", "#haul"] },
  { hook: `3 reasons I'm never going back after the ${p.title}`, script: `One: it actually works. Two: it looks great. Three: ${p.price ? `it's ${p.price}` : "the price is unreal"}. ${p.description.slice(0, 100)} Seriously, try it.`, caption: `3 reasons you need the ${p.title}`, hashtags: ["#gamechanger", "#productreview", "#shopsmall"] },
];

export async function generateCopy(p: Project["product"], count: number): Promise<Omit<Ad, "status" | "id">[]> {
  let copies: Copy[] = [];
  const key = process.env.OPENAI_API_KEY;
  if (key) {
    try {
      const r = await fetch("https://api.openai.com/v1/chat/completions", {
        method: "POST",
        headers: { "content-type": "application/json", authorization: `Bearer ${key}` },
        signal: AbortSignal.timeout(45000),
        body: JSON.stringify({
          model: process.env.OPENAI_MODEL || "gpt-4o-mini",
          response_format: { type: "json_object" },
          messages: [
            { role: "system", content: "You write high-converting 15-second TikTok/Reels UGC ad scripts spoken by a creator to camera. Casual, specific, no hype clichés, no false claims, no medical claims. Return JSON: {\"ads\":[{\"hook\":string (max 12 words, first 2 seconds),\"script\":string (35-45 spoken words),\"caption\":string,\"hashtags\":string[3-5]}]}" },
            { role: "user", content: `Write ${count} distinct ads with different angles for:\nProduct: ${p.title}\nPrice: ${p.price ?? "n/a"}\nDescription: ${p.description.slice(0, 1200)}` },
          ],
        }),
      });
      if (r.ok) copies = JSON.parse((await r.json()).choices[0].message.content).ads ?? [];
    } catch {}
  }
  const fallback = templates(p);
  return Array.from({ length: count }, (_, i) => ({ ...(copies[i] ?? fallback[i % fallback.length]), avatar: AVATARS[i % AVATARS.length] }));
}
