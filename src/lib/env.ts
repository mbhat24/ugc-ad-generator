const prod = process.env.NODE_ENV === "production";

const REQUIRED_PROD = ["DATABASE_URL", "NEXT_PUBLIC_SUPABASE_URL", "NEXT_PUBLIC_SUPABASE_ANON_KEY", "APP_URL"] as const;
const PAIRS: [string[], string][] = [
  [["STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET", "STRIPE_PRICE_STARTER", "STRIPE_PRICE_GROWTH", "STRIPE_PRICE_SCALE"], "Stripe"],
  [["S3_BUCKET", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY"], "S3 storage"],
  [["HIGGSFIELD_KEY_ID", "HIGGSFIELD_KEY_SECRET", "HIGGSFIELD_MODEL"], "Higgsfield"],
];

export function configProblems(env: NodeJS.ProcessEnv = process.env) {
  const problems: string[] = [];
  if (prod) {
    for (const k of REQUIRED_PROD) if (!env[k]) problems.push(`${k} is required in production`);
    if (env.APP_URL && !env.APP_URL.startsWith("https://")) problems.push("APP_URL must be https in production");
  }
  if (env.S3_BUCKET && env.S3_PRIVATE === "0" && !env.S3_PUBLIC_URL) problems.push("S3_PUBLIC_URL is required when S3_PRIVATE=0");
  for (const [keys, name] of PAIRS) {
    const set = keys.filter((k) => env[k]);
    if (set.length && set.length < keys.length) problems.push(`${name} is partly configured, missing: ${keys.filter((k) => !env[k]).join(", ")}`);
    if (prod && !set.length && name !== "Higgsfield") problems.push(`${name} is not configured`);
  }
  return problems;
}

export function assertConfig() {
  const p = configProblems();
  if (p.length) throw new Error(`Invalid configuration:\n- ${p.join("\n- ")}`);
}
