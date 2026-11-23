# AdReel Architecture Documentation

End-to-end architecture for AdReel, an AI UGC and short-form video ad generator for e-commerce.

Legend used throughout: **[Implemented]** exists in this repo, **[Planned]** designed but not built, **[Assumption]** a decision or number that must be validated.

| # | Document | Contents |
|---|---|---|
| 01 | [Requirements and context](01-requirements.md) | Product scope, personas, functional and non-functional requirements, capacity model |
| 02 | [System architecture](02-system-architecture.md) | C4 context/container/component views, sequence diagrams, state machines |
| 03 | [Data architecture](03-data-architecture.md) | Schema, ER diagram, ledger design, consistency, retention, partitioning |
| 04 | [Video generation pipeline](04-video-pipeline.md) | Provider abstraction, job lifecycle, captions, storage, failure handling |
| 05 | [Security and compliance](05-security-compliance.md) | Threat model, controls, privacy, content policy, payments |
| 06 | [Infrastructure and operations](06-infrastructure-operations.md) | Environments, deployment, observability, SLOs, DR, cost model |
| 07 | [Scaling strategy](07-scaling.md) | Stage-by-stage plan from 0 to 100M+ users, bottlenecks, cost limits |
| 08 | [Architecture decision records](08-adrs.md) | Key decisions, alternatives, consequences |
| - | [Roadmap](../ROADMAP.md) | Prioritized future work |

Status honesty: the v1 code builds and lints, but the integrations (Supabase, Stripe, S3/R2, Higgsfield) have not been exercised against live accounts.
