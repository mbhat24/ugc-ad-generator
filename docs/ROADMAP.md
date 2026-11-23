# Roadmap and future work

## Status: v1 (this branch)
Built but not yet verified end-to-end against live Supabase, Stripe, S3/R2 and Higgsfield accounts.

## P0 - make v1 production-ready
- [ ] Run against real Supabase/Stripe/R2; fix integration issues
- [ ] Verify Higgsfield endpoint, auth header and response shape; add webhook support if offered
- [ ] Integration tests for credit debit/refund, webhook idempotency, worker state machine
- [ ] Email confirmation + password reset UX pages
- [ ] Custom domain, TLS, `APP_URL`, Stripe live mode
- [ ] CI (lint, typecheck, build, tests) on GitHub Actions

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
- [ ] Moderation: block restricted products/claims; copy and video policy checks
- [ ] Row Level Security policies in Supabase; least-privilege DB roles
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
