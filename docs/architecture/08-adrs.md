# 08. Architecture decision records

Format: context, decision, alternatives, consequences. Status: accepted unless noted.

## ADR-001 Next.js full-stack app with a separate worker
Context: need fast product iteration and server-rendered UI plus long-running media work.
Decision: Next.js (App Router) for UI and API; a separate Node worker for jobs.
Alternatives: serverless-only (poor fit for ffmpeg and long jobs); microservices from day one (too heavy).
Consequences: one codebase and language; web and worker scale independently; worker needs its own image and deployment.

## ADR-002 Postgres as both database and job queue (pg-boss)
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

## ADR-005 Direct SQL with `pg` instead of an ORM
Decision: thin query helpers and explicit SQL.
Alternatives: Prisma/Drizzle.
Consequences: fewer dependencies and full control of transactions/locks; no generated types or migrations tooling yet. Revisit: adopt Drizzle with versioned migrations before production data (roadmap P0).

## ADR-006 Polling for provider status inside the worker
Context: provider webhook support is unverified.
Decision: start job, requeue with delay, poll.
Alternatives: long-held worker job (wastes slots); webhooks (preferred when available).
Consequences: simple and robust; adds poll traffic and up to ~8 s latency. Add webhooks as accelerator, keep polling as fallback.

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
Consequences: simple playback and ad-platform upload; URLs are unguessable but not authenticated. Planned: signed URLs for sensitive tiers.

## ADR-011 Start monolithic, split by measured need
Decision: no microservices until a component's scaling or ownership demands it. Likely first split: media pipeline service, then billing service.
