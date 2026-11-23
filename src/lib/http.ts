export const fail = (error: string, status: number, extra: Record<string, unknown> = {}) => Response.json({ error, ...extra }, { status });

export function badOrigin(req: Request) {
  const origin = req.headers.get("origin");
  if (!origin) return false;
  try {
    const o = new URL(origin).host;
    const allowed = new Set([req.headers.get("host"), process.env.APP_URL ? new URL(process.env.APP_URL).host : null].filter(Boolean));
    return !allowed.has(o);
  } catch { return true; }
}

export async function readJson(req: Request, maxBytes = 32 * 1024): Promise<Record<string, unknown> | null> {
  const text = await req.text();
  if (text.length > maxBytes) return null;
  try { const v = JSON.parse(text); return v && typeof v === "object" ? v : null; } catch { return null; }
}
