# 02. System architecture

## 1. C4 level 1: system context
```mermaid
flowchart LR
  user[Merchant / Marketer]
  admin[Operator]
  subgraph AdReel
    sys[AdReel platform]
  end
  user -->|HTTPS| sys
  admin -->|Dashboards, Stripe, Supabase consoles| sys
  sys -->|Auth, Postgres| supa[(Supabase)]
  sys -->|Subscriptions, invoices| stripe[Stripe]
  sys -->|Video generation| hf[Higgsfield]
  sys -->|Copywriting| oai[OpenAI]
  sys -->|Fetch product pages| web[(Merchant stores)]
  sys -->|Video objects| s3[(S3 / R2 + CDN)]
  stripe -->|Webhooks| sys
```

## 2. C4 level 2: containers
```mermaid
flowchart TB
  subgraph Client
    br[Browser: Next.js React UI]
  end
  subgraph Edge
    cdn[CDN for static + videos]
  end
  subgraph Compute
    web[web: Next.js standalone, stateless]
    worker[worker: Node + ffmpeg + tsx]
  end
  subgraph Data
    pg[(Postgres: app tables + pgboss schema)]
    obj[(Object storage)]
  end
  br --> cdn --> web
  br -->|video playback| cdn --> obj
  web -->|SQL| pg
  web -->|enqueue| pg
  worker -->|dequeue, update| pg
  worker -->|start/poll| hf[Higgsfield API]
  worker -->|upload| obj
  web -->|auth| supa[Supabase Auth]
  web -->|checkout/portal| stripe[Stripe]
  stripe -->|webhook| web
```
**Notes**
- `web` holds no state; any instance can serve any request. Sessions are cookies validated against Supabase.
- Postgres is both system of record and queue (pg-boss). This keeps v1 operationally small; see ADR-002 for the migration path.
- Workers are independently scaled; a crashed worker's job is retried by the queue.

## 3. C4 level 3: web component view
| Component | Path | Depends on |
|---|---|---|
| Proxy (session refresh) | `src/proxy.ts` | Supabase SSR |
| Auth/profile | `src/lib/auth.ts`, `src/lib/supabase.ts` | Supabase, db |
| Project API | `src/app/api/projects` | auth, scrape, script, ratelimit, queue, db |
| Billing API | `src/app/api/billing/*`, `src/app/api/stripe/webhook` | stripe lib, db |
| Scraper | `src/lib/scrape.ts` | dns, net, fetch |
| Copy generator | `src/lib/script.ts` | OpenAI (optional) |
| Data access | `src/lib/db.ts` | pg |
| Queue producer | `src/lib/queue.ts` | pg-boss |

Worker components: queue consumer (`worker/index.ts`), job state machine (`src/lib/jobs.ts`), provider (`src/lib/video`), captions (`src/lib/captions.ts`), storage (`src/lib/storage.ts`).

## 4. Sequence: generate ads
```mermaid
sequenceDiagram
  actor U as User
  participant W as web
  participant DB as Postgres
  participant Q as pg-boss
  participant K as worker
  participant H as Higgsfield
  participant S as S3/R2
  U->>W: POST /api/projects {url,count}
  W->>DB: balance, usage limits
  W->>W: scrape product (SSRF-safe), generate copy
  W->>DB: One transaction: lock profile row, recheck balance, insert project and ads, debit ledger per ad
  W->>Q: send(ad-generate, adId) x N
  W-->>U: 200 {projectId}
  loop every ~8s until done
    Q->>K: job(adId)
    K->>H: start (first) or poll
    K->>Q: requeue with startAfter
  end
  K->>H: fetch finished video
  K->>K: ffmpeg burn captions
  K->>S: upload videos/{user}/{ad}.mp4
  K->>DB: ad.status=done, video_url
  U->>W: GET /api/projects/:id (poll 4s)
  W-->>U: status + video_url
```

## 5. Sequence: subscribe
```mermaid
sequenceDiagram
  actor U as User
  participant W as web
  participant St as Stripe
  participant DB as Postgres
  U->>W: POST /api/billing/checkout {plan}
  W->>St: create customer (once) + Checkout Session
  W-->>U: redirect URL
  U->>St: pay
  St->>W: webhook customer.subscription.created
  W->>DB: profiles.plan = f(price id)
  St->>W: webhook invoice.paid
  W->>St: retrieve subscription
  W->>DB: ledger +N (reason=subscription, ref=invoice id) ON CONFLICT DO NOTHING
```
Webhook handlers are idempotent: plan update is a pure set, grants are keyed by invoice id.

## 6. Ad state machine
```mermaid
stateDiagram-v2
  [*] --> pending: created, credit debited
  pending --> processing: provider job started
  processing --> processing: poll, still running
  processing --> done: video finalized
  pending --> failed: start error / timeout
  processing --> failed: provider failed / timeout / error
  failed --> [*]: credit refunded (idempotent)
  done --> [*]
```
Transitions are guarded by `WHERE status IN (...)` updates so duplicate job deliveries cannot double-refund or overwrite a finished ad.

## 7. Failure modes and handling
| Failure | Effect | Mitigation |
|---|---|---|
| Worker crash mid job | Job expires and retries | pg-boss retry/backoff; steps idempotent |
| Duplicate job delivery | Double processing | Status-guarded updates; unique refund key |
| Provider outage | Ads fail | Fail fast + refund; [Planned] failover provider, circuit breaker |
| Storage outage | No captions/upload | Falls back to provider URL; [Planned] retry/re-host |
| ffmpeg error | No captions | Falls back to uncaptioned video |
| Stripe webhook delay/replay | Late/duplicate grants | Idempotency keys; retrieve-from-source |
| DB failover | Brief errors | Retry on client; pooled connections; managed HA |
| Scrape target slow/hostile | Request stalls | 12 s timeout, redirect cap, private-IP block |
| Queue enqueue fails after commit | Paid ad never starts | Compensating fail + refund (implemented); [Planned] outbox reconciler |

## 8. API surface (v1)
| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | /api/auth/signup, login, logout | public/session | Supabase wrappers |
| POST | /api/projects | session | Create project and enqueue ads |
| GET | /api/projects/:id | session, owner | Project with ads and status |
| POST | /api/billing/checkout | session | Stripe Checkout URL |
| POST | /api/billing/portal | session | Stripe portal URL |
| POST | /api/stripe/webhook | Stripe signature | Plan and credit sync |

[Planned] versioned public API with API keys, per-key rate limits and outbound webhooks for agencies.
