/** CSRF defence for cookie-authenticated mutating requests: the Origin header, when present, must be ours. */
export function sameOrigin(req: Request) {
  const origin = req.headers.get("origin");
  if (!origin) return true;
  try {
    const host = new URL(origin).host;
    return host === req.headers.get("host") || (!!process.env.APP_URL && host === new URL(process.env.APP_URL).host);
  } catch { return false; }
}
