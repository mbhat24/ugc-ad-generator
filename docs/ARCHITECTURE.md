# Architecture (v1)

## Overview
AdReel turns a product URL into short-form UGC video ads (9:16) with AI avatars, hooks, scripts and captions.

```
Browser ──> Next.js web (stateless) ──> Postgres (Supabase)
                 │                          ▲   ▲
                 │ enqueue (pg-boss)        │   │ ledger, projects, ads
                 ▼                          │   │
              Postgres queue ──> Worker ────┘   │
                                  │             │
                    Higgsfield API (start/poll) │
                                  │             │
                    ffmpeg captions ──> S3/R2 ──┘ (public video URL)
Stripe ──webhook──> /api/stripe/webhook ──> plan + credit grants
```

## Components
| Component | Location | Responsibility |
|---|---|---|
| Web app | `src/app` | Pages, API routes, auth gate, project creation, billing endpoints |
| Auth | `src/lib/auth.ts`, `src/lib/supabase.ts`, `src/proxy.ts` | Supabase Auth sessions (cookies), session refresh in proxy, profile upsert |
| Data layer | `src/lib/db.ts`, `src/lib/schema.ts`, `src/lib/migrate.ts` | pg pool, transactions, schema (idempotent), `npm run migrate` |
| Credits | `credit_ledger` table | Balance = SUM(delta). Unique (reason, ref) gives idempotent grants/refunds |
| Billing | `src/lib/stripe.ts`, `src/app/api/billing/*`, `src/app/api/stripe/webhook` | Checkout, portal, subscription sync, credit grant on `invoice.paid` |
| Scraper | `src/lib/scrape.ts` | Product extraction (Shopify JSON, OpenGraph), SSRF guard (DNS + redirect validation) |
| Copy | `src/lib/script.ts` | Hooks/scripts/captions via OpenAI, template fallback |
| Queue | `src/lib/queue.ts` | pg-boss singleton, queue `ad-generate`, retries with backoff |
| Worker | `worker/index.ts`, `src/lib/jobs.ts` | Start/poll video job, finalize, refund on failure |
| Video | `src/lib/video` | Provider interface: Higgsfield and mock |
| Captions | `src/lib/captions.ts` | SRT generation + ffmpeg burn-in |
| Storage | `src/lib/storage.ts` | S3-compatible upload (R2/S3/MinIO) |
| Limits | `src/lib/ratelimit.ts` | Per-user hourly projects, daily ads |

## Data model
- `profiles(id, email, plan, stripe_customer_id)` - id equals Supabase auth user id
- `credit_ledger(user_id, delta, reason, ref)` - reasons: `signup`, `subscription`, `ad`, `refund`
- `projects(user_id, url, product jsonb)`
- `ads(project_id, position, hook, script, caption, hashtags, avatar, status, video_job_id, video_url, error)`
- pg-boss tables in the `pgboss` schema

## Key flows
**Generate:** POST `/api/projects` -> auth -> credit check -> rate limit -> scrape -> copy -> one transaction (lock profile row, re-check balance, insert project/ads, debit ledger) -> enqueue one job per ad.
**Worker job:** no provider job yet -> `provider.start`, store id, requeue in 8s. Else poll; processing -> requeue; failed/timeout (25 min) -> mark failed + refund; done -> download, burn captions, upload, mark done (falls back to provider URL if storage/ffmpeg fails).
**Billing:** Checkout -> webhook `customer.subscription.*` sets plan from price id; `invoice.paid` grants plan credits (idempotent by invoice id).

## Deployment
Two containers from one Dockerfile: `web` (Next standalone) and `worker` (ffmpeg + tsx). Postgres/Auth on Supabase, video on R2/S3. Scale web and worker replicas independently; workers coordinate via `SKIP LOCKED`.

## Security
- Supabase-managed auth; httpOnly cookies; ownership checks on every project read
- SSRF protection on scraping (private/loopback/link-local blocked, redirects re-validated)
- Stripe webhook signature verification
- Per-user generation caps to bound provider spend
- Known gaps: see ROADMAP "Hardening"

## Scaling notes
| Stage | Actions |
|---|---|
| 0-10k users | Current design; Supabase pooler; 1-2 web, 1-3 workers |
| 10k-100k | Read replicas for dashboards, CDN in front of R2, per-plan worker concurrency, queue metrics/alerts |
| 100k-1M+ | Dedicated queue (SQS/Kafka) or sharded pg-boss, partition `ads`/`credit_ledger`, cache product scrapes, multi-region web, provider failover |
| Cost | Provider cost per video is the real limit; enforce margins via credit pricing and caps |
