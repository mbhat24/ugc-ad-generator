# Roadmap and future work

## Status: v1 (this branch)
Built but not yet verified end-to-end against live Supabase, Stripe, S3/R2 and Higgsfield accounts.

## P0 - make v1 production-ready
- [ ] Run against real Supabase/Stripe/R2; fix integration issues
- [ ] Pick the Higgsfield model in the console and match `HIGGSFIELD_MODEL` and the request body to its schema; test with a real account (auth, status lifecycle, idempotency, 429 and webhook handling are implemented per the official docs)
- [ ] Integration tests for credit debit/refund, webhook idempotency, worker state machine
- [ ] Email confirmation + password reset UX pages
- [ ] Custom domain, TLS, `APP_URL`, Stripe live mode
- [x] CI (lint, typecheck, migrate, tests, build) on GitHub Actions: `.github/workflows/ci.yml`
- [x] Versioned migrations, health/readiness endpoints, per-IP rate limits, security headers, reconciler + dead-letter queue, account export/deletion, Stripe event dedupe, config validation

## P1 - product
- [ ] Multiple product images and avatar/voice selection; avatar library
- [ ] Hook A/B variants, regenerate single ad, edit script before generating
- [ ] Caption styles (karaoke-style word highlight), brand colors/logo
- [ ] Direct publish/export to TikTok, Reels, Meta Ads; bulk ZIP download
- [ ] Shopify app / store connect; auto-generate for new products
- [ ] Credit top-ups and annual plans; coupons/referrals
- [ ] Video thumbnails and in-app trimming

## P2 - hardening
- [ ] Rate limiting by IP (auth and API), bot protection (Turnstile)
- [ ] DNS-rebinding-proof scraper (pin resolved IP) and sandboxed scraper worker
- [x] Moderation: keyword category blocks, claim scrubbing, optional OpenAI moderation; [ ] image/video-level checks
- [x] RLS enabled on all tables, anon/authenticated revoked (migration 003); [ ] dedicated least-privilege web DB role (see launch checklist)
- [ ] Audit log, admin dashboard, abuse tooling
- [ ] Credit expiry/rollover policy per plan
- [ ] Secrets management, key rotation

## P3 - scale and ops
- [ ] Observability: Sentry, structured logs, OpenTelemetry, queue depth alerts
- [ ] Autoscale workers on queue depth; priority queues per plan
- [ ] Read replicas, partitioning, archival of old projects
- [ ] Multi-provider video failover (cost/latency routing)
- [ ] Scrape cache and CDN for product images
- [ ] Multi-region and disaster recovery runbooks
- [ ] Load tests and cost dashboards (cost per video, gross margin)

## Hackathon (Higgsfield) checklist
- [ ] Real Higgsfield generation working with demo store
- [ ] 60-second demo video and landing page polish
- [ ] Public deploy and post on X
