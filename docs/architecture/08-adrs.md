# 08. Architecture decision records

Format: context, decision, alternatives, consequences. Status: accepted unless noted.

## ADR-001 Next.js full-stack app with a separate worker (superseded by ADR-012)
Context: need fast product iteration and server-rendered UI plus long-running media work.
Decision: Next.js (App Router) for UI and API; a separate Node worker for jobs.
Alternatives: serverless-only (poor fit for ffmpeg and long jobs); microservices from day one (too heavy).
Consequences: one codebase and language; web and worker scale independently; worker needs its own image and deployment.

## ADR-002 Postgres as both database and job queue (pg-boss; superseded by ADR-013, the principle stands)
Context: want minimal infrastructure and transactional enqueue.
Decision: pg-boss on the same Postgres.
Alternatives: Redis/BullMQ (extra system, weaker durability by default); SQS (cloud lock-in); Temporal (powerful, heavy).
Consequences: simple ops and atomic enqueue possible; queue load shares the DB. Revisit at S3 (doc 07): move queue to its own DB or SQS/Temporal. Job handlers are idempotent so the swap is mechanical.

## ADR-003 Supabase for Auth and managed Postgres; app in containers
Context: avoid building auth; keep deploy portability.
Decision: Supabase cloud for Auth and Postgres; web and worker in Docker on any container host.
Alternatives: Clerk (faster, per-MAU cost), Auth.js (self-managed), self-hosted Postgres.
Consequences: less auth code to secure; coupling to Supabase Auth APIs; direct `pg` access keeps data layer portable.

## ADR-004 Append-only credit ledger
Context: credits are money-like; need auditability and idempotency.
Decision: ledger rows with `(reason, ref)` uniqueness; balance is a sum.
Alternatives: mutable counter (race-prone, no audit); per-period quota fields (hard to refund exactly once).
Consequences: correct under retries and replays; read cost grows with rows, solved later by a balances table.

## ADR-005 Direct SQL instead of an ORM (now pgx in Go)
Decision: thin query helpers and explicit SQL.
Alternatives: Prisma/Drizzle.
Consequences: fewer dependencies and full control of transactions/locks; no generated types or migrations tooling yet. Revisit: adopt Drizzle with versioned migrations before production data (roadmap P0).

## ADR-006 Webhook-accelerated polling for provider status
Context: Higgsfield supports signed webhooks (retried up to 2 hours, duplicates possible) and polling, and recommends polling as the fallback.
Decision: worker starts the job, requeues with delay and polls; an optional verified webhook only triggers an immediate re-poll.
Alternatives: long-held worker job (wastes slots); trusting webhook payload directly (forgery/replay risk, ordering issues).
Consequences: correct even if webhooks are lost or forged; poll traffic remains and latency is up to ~8 s without webhooks.

## ADR-007 Provider abstraction with mock implementation
Decision: `VideoProvider` interface and environment-selected implementation.
Consequences: demo and tests without cost; future multi-provider routing is additive.

## ADR-008 Captions burned in server-side with ffmpeg
Alternatives: client-side overlay (not downloadable); provider-native captions (uncontrolled).
Consequences: deterministic output for download and upload to ad platforms; worker needs ffmpeg and CPU. Timing is even-split in v1 (approximate); accurate alignment is planned.

## ADR-009 Stripe subscriptions mapped to credit grants
Decision: plan derived from price id; credits granted on `invoice.paid`.
Alternatives: Stripe metered billing (variable invoices, harder UX); one-time packs only.
Consequences: predictable revenue and simple UX; no rollover logic yet; plan changes mid-cycle rely on Stripe proration and next-invoice grants.

## ADR-010 Object storage with public CDN URLs
Decision: public immutable URLs for finished videos.
Consequences (revised): superseded by private storage with short-lived signed URLs by default; public mode remains an opt-out (`S3_PRIVATE=0`).

## ADR-011 Start monolithic, split by measured need
Decision: no microservices until a component's scaling or ownership demands it. Likely first split: media pipeline service, then billing service.

## ADR-012 Standalone Go backend; Next.js reduced to UI plus BFF
Context: the product is a long-lived system with background processing, money-like state and external integrations. The first version kept all server code inside the Next.js app, coupling the API to a fast-moving UI framework and making a non-web client (mobile, partner API) awkward.
Decision: a Go backend (`backend/`) owns all business logic and exposes a versioned REST API described by OpenAPI. Next.js keeps rendering and the session cookie and proxies `/api/v1/*` with a bearer token (BFF).
Alternatives: keep TypeScript and extract a NestJS/Fastify service (less rewriting, weaker runtime characteristics for the worker); Python/FastAPI (strong media/AI ecosystem, slower, more operational care); Java/Kotlin (heavier).
Consequences: static binaries and small images, strong concurrency for polling workers, strict typing at the API boundary, independent deploy and scaling of web, api and worker. Costs: two languages in the repo (types shared through OpenAPI codegen) and a rewrite.

## ADR-013 River as the job queue
Context: needs retries, scheduling, uniqueness and, above all, enqueueing inside the business transaction.
Decision: River on the existing Postgres (pgx driver). Jobs are inserted with `InsertTx` in the transaction that debits credits; polling uses `JobSnooze` (does not consume attempts); `ErrorHandler` refunds when attempts are exhausted; a periodic job reconciles stale ads.
Alternatives: keep pg-boss (Node only); Redis/Asynq (extra system, no transactional enqueue); Temporal (powerful, heavy, revisit at scale).
Consequences: exactly the transactional-outbox guarantee we wanted without extra infrastructure; the queue shares Postgres capacity (see doc 07 for the exit path).

## ADR-014 Backend-for-frontend proxy with httpOnly cookies
Decision: tokens never reach browser JavaScript. The Next.js route handler reads the session cookie, adds the bearer token and forwards the request; mutating cookie-authenticated requests require a same-origin `Origin`.
Consequences: no CORS and no token storage in the browser; one more hop (negligible); webhook endpoints are forwarded unauthenticated and verified by signature in the API (or point providers straight at the API via `API_PUBLIC_URL`).

## ADR-015 SSRF defence at connect time
Decision: the scraper resolves DNS itself, rejects any non-public address and dials the validated IP, with no proxy support and a redirect cap. Because validation happens in the dialer, DNS rebinding and redirects to internal hosts cannot bypass it.
