export const devAuth = !process.env.NEXT_PUBLIC_SUPABASE_URL && process.env.NODE_ENV !== "production";
export const DEV_COOKIE = "dev_token";
