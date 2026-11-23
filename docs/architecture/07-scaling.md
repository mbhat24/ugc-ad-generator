# 07. Scaling strategy

Principle: do not build for 100M users on day one. Build a design whose bottlenecks are known and whose next step is cheap, and measure before each step. The first real limit is provider cost and quota, not infrastructure.

## 1. Stage plan
| Stage | Users (registered) | Videos/day [Assumption] | Architecture |
|---|---|---|---|
| S0 prototype | < 100 | < 100 | v1 as built: 1 web, 1 worker, single Postgres |
| S1 launch | 100 - 10k | 100 - 5k | 2+ web, 2-5 workers, Supabase Pro, R2, Sentry, CI/CD |
| S2 growth | 10k - 100k | 5k - 50k | Autoscaling, read replica, CDN tuning, queue metrics, provider router, rate limits per IP |
| S3 scale | 100k - 1M | 50k - 500k | Dedicated queue DB or SQS, partitioned ledger/ads, multi-provider contracts, warm DR region |
| S4 hyperscale | 1M - 100M+ | 500k+ | Service split, event streaming, sharded/regional data, dedicated media pipeline, own GPU capacity or committed provider capacity |

## 2. Bottlenecks by layer and remedies
| Layer | First symptom | Remedy |
|---|---|---|
| Web CPU | High p95 under load | Add replicas; cache static/PPR; move heavy work to worker |
| Postgres connections | Pool exhaustion | Pooler (pgbouncer/Supavisor), smaller pools, read replicas, separate DB for queue |
| Postgres writes | Ledger/ads write contention | Partition by time, batch inserts, balances table, shard by `user_id` hash |
| Queue (pg-boss) | Poll load, bloat, latency | Raise poll interval, dedicated queue DB, then SQS/Kafka/Temporal |
| Workers | ffmpeg CPU | Horizontal scale, CPU-optimized nodes, GPU/hardware encode, separate caption service |
| Provider | 429s, long queues | Quotas, multi-provider routing, priority by plan, backpressure to users, committed-use deals |
| Storage/egress | Cost and latency | CDN, lifecycle tiers, expire free-tier videos, HLS |
| Scraper | Blocks, slow pages | Per-domain concurrency limits, proxy pool, cache, headless pool |
| Stripe webhooks | Backlog | Fast ack, enqueue handling, idempotent consumers |

## 3. Multi-tenancy and fairness
Per-user and per-plan concurrency caps; priority queues so paid jobs run before free; fair scheduling to stop one agency from starving others; separate queues per provider so one provider's slowness does not block another.

## 4. Data scale
- Shard key: `user_id` (all hot queries are per user). Start with partitioning, then Citus or application sharding only if single-primary writes saturate.
- Move analytics off OLTP: change data capture to a warehouse (BigQuery/ClickHouse) for BI.
- Archive: cold projects/videos to cheaper tiers after N days.

## 5. Global distribution
Edge-cached marketing pages and video via CDN; regional web deployments reading from replicas with writes to primary; data residency options (EU) as a later enterprise feature via regional stacks.

## 6. Economics at scale
At large volume provider cost per video dominates. Levers: negotiated rates and committed capacity, routing to cheaper models for easy cases, shorter clips, caching reusable avatar/segments, selective regeneration, and eventually self-hosted open models on reserved GPUs once utilization justifies it. Revenue-side levers: credit packs, annual plans, enterprise contracts.

## 7. Load testing plan [Planned]
Targets: sustain 10x expected peak creation rate with provider mocked; verify queue drain time, ledger correctness under concurrency (property test: balance never negative, no double refund), webhook storm replay, worker crash/chaos. Run in staging with synthetic provider that has configurable latency and failure rate.

## 8. Capacity guardrails
Alert and auto-throttle when daily provider spend exceeds budget; global kill switch per provider; per-user spend ceiling regardless of plan.
