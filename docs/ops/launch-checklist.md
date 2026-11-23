# Launch checklist: from this repo to first paying customers

Work top to bottom. Each step lists what to do and how to verify. Items marked (code) are already implemented; the rest need your accounts.

## 1. Accounts and keys
- [ ] **Supabase project** (region near your users). Settings: Authentication > Providers > Email enabled, **Confirm email ON** (the app rejects unconfirmed users in production), configure custom SMTP (the default sender is rate limited), set Site URL and Redirect URLs to your `APP_URL`.
- [ ] Copy `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY`, `SUPABASE_SERVICE_ROLE_KEY` (server only), and the **session pooler** connection string as `DATABASE_URL`.
- [ ] **Stripe**: create 3 recurring monthly prices (Starter, Growth, Scale), set `STRIPE_PRICE_*`; `STRIPE_SECRET_KEY`; webhook endpoint `https://<domain>/api/stripe/webhook` with events `customer.subscription.created|updated|deleted`, `invoice.paid`, `invoice.payment_failed`; set `STRIPE_WEBHOOK_SECRET`. Enable the customer portal (plan switching, cancellation). Consider Stripe Tax.
- [ ] **R2/S3 bucket**, private (`S3_PRIVATE` unset = signed URLs, 1 hour expiry (code)). Create an API token limited to this bucket. Set `S3_BUCKET`, `S3_ENDPOINT`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`. Add a lifecycle rule to expire objects after your retention period.
- [ ] **Higgsfield**: create credentials; in console.higgsfield.ai choose the model, copy its endpoint path to `HIGGSFIELD_MODEL`, and **match the request body in `src/lib/video/index.ts` to that model's schema**. Reveal the webhook signing secret (`whsec_...`) into `HIGGSFIELD_WEBHOOK_SECRET`. Ask Higgsfield for the concurrency limit you need.
- [ ] Optional: `OPENAI_API_KEY` (better scripts + moderation endpoint), `ERROR_WEBHOOK_URL` (Slack/Discord-compatible alerts for error logs (code)).
- [ ] Rotate/revoke any token that has ever been pasted into chat or committed.

## 2. Database
- [ ] `DATABASE_URL=... npm run migrate` (applies `migrations/001-003`; RLS is enabled so the Supabase anon key cannot read tables (code)).
- [ ] Verify in Supabase SQL editor: `select relname, relrowsecurity from pg_class where relname in ('profiles','ads','projects','credit_ledger');` all true.
- [ ] Enable daily backups and point-in-time recovery; run one restore test.
- [ ] Optional hardening: create a dedicated role for the web tier with only SELECT/INSERT/UPDATE/DELETE on the app tables and use it in `DATABASE_URL` for `web`, keeping the owner role for migrations and the worker.

## 3. Deploy
- [ ] Build images: `docker build --target web -t adreel-web --build-arg NEXT_PUBLIC_SUPABASE_URL=... --build-arg NEXT_PUBLIC_SUPABASE_ANON_KEY=... .` and `docker build --target worker -t adreel-worker .`
- [ ] Run **web** (2+ replicas) and **worker** (2+ replicas) on Fly.io, Railway, Render or ECS. All env vars from `.env.example` go to both. `NODE_ENV=production` and `APP_URL=https://...` are mandatory (the app refuses to boot with missing or partial config (code)).
- [ ] Health checks: web `GET /api/ready`; alert if it fails.
- [ ] Put a CDN/WAF (Cloudflare) in front; set `TRUSTED_PROXY_HOPS` to the number of proxies so rate limits see real client IPs.
- [ ] Custom domain + TLS; HSTS is sent in production (code).

## 4. Pre-launch verification (do in staging with test-mode keys first)
- [ ] Sign up with a real inbox, confirm email, log in.
- [ ] Generate a video end to end; confirm captions render, the signed URL plays and downloads, and the Higgsfield copy is deleted afterwards.
- [ ] Subscribe with Stripe test card `4242 4242 4242 4242`; verify plan and credits update; replay the webhook from the Stripe dashboard and confirm no duplicate credits.
- [ ] Cancel in the portal; verify the plan returns to free at the right time.
- [ ] Force a failure (invalid model path) and confirm the credit is refunded.
- [ ] Kill the worker mid-job and confirm the reconciler resumes it within ~3 minutes.
- [ ] Export and delete a test account.
- [ ] Run `BASE=https://staging... USERS=50 node scripts/loadtest.mjs` against a staging stack using the mock provider, then a small real batch to confirm provider limits.

## 5. Business and legal
- [ ] Fill the bracketed placeholders in `/terms`, `/privacy`, `/acceptable-use` and have counsel review them.
- [ ] Add your support email, refund policy, company details; register the business entity and Stripe account for your jurisdiction.
- [ ] Confirm each provider's DPA, data retention and no-training terms; list them as subprocessors.
- [ ] Decide pricing from real numbers: `credit price >= provider cost per video / (1 - target margin)`; recompute after your first 100 videos.
- [ ] Platform rules: label AI-generated content; do not sell fake-testimonial positioning.

## 6. Operations from day one
- [ ] Error alerts (`ERROR_WEBHOOK_URL` or Sentry), uptime monitor on `/api/ready`, log shipping from containers (logs are JSON).
- [ ] Daily check: failed-ad rate, queue age (`select now()-min(created_at) from pgboss.job where state='created'`), provider spend vs budget.
- [ ] Spend guardrails: tune `MAX_PROJECTS_PER_HOUR`, `MAX_ADS_PER_DAY`, `SIGNUPS_PER_IP_HOUR`; keep the Higgsfield account spend limit set on their side.
- [ ] Incident basics: how to pause generation (scale worker to 0, jobs wait in queue), how to refund (insert a `refund` ledger row with a unique ref).
