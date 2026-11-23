# 01. Requirements and context

## 1. Problem and value
D2C and Shopify merchants need a steady stream of short-form video creatives (TikTok, Reels, Shorts) to feed paid and organic channels. Traditional UGC costs money per creator, needs shipped samples and takes days to weeks. AdReel produces test-ready creatives from a product URL in minutes at a fraction of the cost.

## 2. Personas
| Persona | Need | Success metric |
|---|---|---|
| Solo store owner | Cheap creatives without learning tools | Time from URL to first usable ad under 10 min |
| Performance marketer | Many hook/angle variants for A/B testing | Variants per product, cost per variant |
| Agency | Multi-brand, volume, white-label, approvals | Throughput, seats, brand workspaces [Planned] |
| Platform admin | Control abuse and spend | Margin per video, abuse rate |

## 3. Scope
**In scope (v1):** URL ingestion, copy generation (hook, script, caption, hashtags), AI avatar video generation via provider, captions burned in, storage and download, subscription plans with credits.
**Out of scope for v1 [Planned]:** direct publishing to ad platforms, team workspaces, custom avatars/voice cloning, editing timeline, A/B result analytics.

## 4. Functional requirements
| ID | Requirement | Status |
|---|---|---|
| F1 | Sign up, log in, email confirmation | Implemented (Supabase Auth) |
| F2 | Submit product URL, extract title, price, description, images | Implemented |
| F3 | Generate 1-3 ad variants per project with distinct angles | Implemented |
| F4 | Generate vertical 9:16 UGC video per variant | Implemented (Higgsfield provider + mock) |
| F5 | Burn captions into video | Implemented (ffmpeg, requires storage) |
| F6 | Show live status, preview, download | Implemented (client polling) |
| F7 | Subscription plans, credit grants on payment, billing portal | Implemented (Stripe) |
| F8 | Credits debited per ad, refunded on failure | Implemented |
| F9 | Regenerate single ad, edit script, choose avatar/voice | Planned |
| F10 | Bulk export, publish to TikTok/Meta | Planned |
| F11 | Teams, roles, brand kits | Planned |

## 5. Non-functional requirements
| Category | Target | Notes |
|---|---|---|
| Availability (web/API) | 99.9% monthly | Stateless web, managed Postgres |
| Video completion | 99% of accepted jobs end in done or failed (never stuck) | Job age cap 25 min, refund on failure |
| Latency | Page TTFB p95 under 500 ms; API p95 under 800 ms excluding scrape | Scrape up to 12 s timeout |
| Generation time | p50 under 5 min, p95 under 15 min | Provider dependent [Assumption] |
| Durability | No lost paid credits or finished videos | Ledger in Postgres, videos in object storage |
| Consistency | Credit balance never negative; exactly-once grants | Row lock + unique idempotency keys |
| Security | OWASP ASVS L2 target | See doc 05 |
| Privacy | GDPR/CCPA-ready deletion and export | Planned workflows |
| Cost | Gross margin target of 60%+ per video [Assumption] | Credit price must exceed provider cost |
| Scalability | Horizontal web and workers; DB scale-up then read replicas | See doc 07 |

## 6. Constraints
- Video generation is the dominant cost and latency. All capacity planning is bounded by provider rate limits and unit cost, not by web servers.
- Third-party providers (Higgsfield, OpenAI, Stripe, Supabase) are external dependencies with their own outages and policies.
- Generated media of people raises content, likeness and disclosure obligations (doc 05).

## 7. Capacity model (illustrative, all inputs are assumptions)
Define: U registered users, a = fraction active per month, v = videos per active user per month, c = provider cost per video.
- Monthly videos V = U * a * v
- Average generation throughput = V / (30 * 86400) videos/s; peak is roughly 5-10x average
- Provider spend per month = V * c
- Concurrent jobs in flight = throughput * mean generation time (Little's law)

Example with U = 1,000,000, a = 10%, v = 8: V = 800,000 videos/month, about 0.31 videos/s average, about 3 videos/s at 10x peak. With a mean 5 min generation time, in-flight jobs average about 93 and peak about 930. This is a queueing and provider-quota problem, not a CPU problem. At U = 100M the same ratios give 100x those numbers, which is why provider contracts, multi-provider routing and cost controls dominate the design.
