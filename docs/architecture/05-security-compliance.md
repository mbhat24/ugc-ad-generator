# 05. Security and compliance

## 1. Assets and trust boundaries
Assets: user accounts, billing relationship, product data, generated videos, provider and payment API keys, credit balances (monetary value).
Boundaries: Internet to web; web/worker to Postgres; web/worker to third parties; Stripe to webhook endpoint; untrusted merchant sites to scraper.

## 2. Threat model (STRIDE summary)
| Threat | Example | Control | Status |
|---|---|---|---|
| Spoofing | Stolen session | httpOnly, SameSite=lax cookies via Supabase SSR; server-side `getUser()` | Implemented |
| Tampering | Forged Stripe event | Signature verification with webhook secret | Implemented |
| Tampering | Forged/replayed Higgsfield webhook | Standard Webhooks signature + timestamp check; webhook only triggers a re-poll, never carries trusted data | Implemented |
| Tampering | Edit others' projects | Ownership predicate in every query | Implemented |
| Repudiation | Dispute credits | Append-only ledger with reasons and refs | Implemented |
| Information disclosure | SSRF to metadata/internal | DNS-resolved private-IP blocking, redirect re-validation | Implemented (rebinding gap noted) |
| Information disclosure | Cross-tenant video access | Unguessable but public URLs | Gap: signed URLs [Planned] |
| DoS | Flood project creation | Per-user caps, Postgres-backed per-IP and per-email limits on login/signup, per-user limits on projects/billing | Implemented; global/WAF limits [Planned] |
| DoS | Giant pages | Body cap, timeouts | Implemented |
| Elevation | Over-privileged DB role | Single privileged role in v1 | Gap: least privilege + RLS [Planned] |
| Cost abuse | Free-tier farming | 1 credit on signup | Partial; email verification, device/IP heuristics, CAPTCHA [Planned] |

## 3. Authentication and sessions
- Supabase Auth: email/password with confirmation; [Planned] OAuth (Google), magic link, MFA for admins and paying users.
- `src/proxy.ts` refreshes session cookies on each request; pages and APIs call `currentUser()` which validates with Supabase (not just decoding the cookie).
- Passwords are never handled by app code (v1 removed the earlier scrypt implementation).

## 4. Authorization
Single-tenant ownership model: `user_id` match on projects/ads. [Planned] organizations with roles (owner, admin, editor, viewer) and brand workspaces; policy checks centralized in one module and mirrored by RLS.

## 5. Secrets and configuration
- All secrets via environment variables; `.env*` is git-ignored, `.env.example` has blanks only.
- [Planned] secret manager (Vault, AWS SM, Doppler), rotation runbook, separate keys per environment, Stripe restricted keys.
- `NEXT_PUBLIC_*` values are public by design (Supabase URL and anon key); never put service keys there.
- A personal access token was once pasted into a chat during development; treat any such token as compromised and revoke it. Enable GitHub secret scanning and push protection on the repo.

## 6. Application security
- Input validation: URL length/scheme, count clamped 1-3, plan whitelist, UUID format check on ids.
- Output: React escapes by default; scraped text is stripped of tags before use.
- Implemented: security headers (X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Permissions-Policy, HSTS in production, CSP limited to frame-ancestors/base-uri/form-action/object-src), Origin checks on every cookie-authenticated mutating route, JSON body size cap, fail-fast configuration validation in production, structured JSON logs.
- [Planned] full script-src CSP with nonces, dependency scanning (Dependabot, `npm audit` in CI), SAST, container image scanning, SBOM.
- Containers run as non-root `node` user; worker image minimal plus ffmpeg.

## 7. Payments and PCI
Card data never touches AdReel; Stripe Checkout and portal keep scope at SAQ A. Webhook endpoint is excluded from session proxy and verifies signatures. Tax (VAT/sales tax) via Stripe Tax [Planned]; invoices and receipts handled by Stripe.

## 8. Privacy
- Data minimization: email, product data, generated content. No analytics PII beyond need.
- Rights: access, export, erasure workflows [Planned] (see doc 03 section 6).
- Subprocessors to disclose: Supabase, Stripe, Higgsfield, OpenAI, storage/CDN provider, hosting provider.
- Product text is sent to OpenAI and Higgsfield; confirm DPAs and no-training terms. Higgsfield documents a model-training opt-out and shorter retention (`Hf-Expires-After`); use both for customer data [Planned]. Provider copies are deleted after we store our own.

## 9. Content and platform policy
- Prohibited categories (weapons, adult, illegal, deceptive health/finance claims) must be blocked pre-generation [Planned classifier + rules].
- Synthetic-media disclosure: TikTok, Meta and YouTube require labeling of realistic AI-generated content; surface a label/watermark option and guidance in-product.
- Avoid generating likeness of real people without consent; use provider-approved synthetic avatars only.
- Terms of Service and Acceptable Use Policy required before public launch; DMCA/takedown contact.

## 10. Logging, detection, response
- Log security-relevant events (auth failures, webhook rejections, limit hits, SSRF blocks) without secrets.
- [Planned] alerting on spikes, anomaly detection on credit usage, incident response runbook with severity levels and customer comms template, annual pen test.

## 11. Compliance roadmap
GDPR/CCPA baseline at launch; SOC 2 Type I then II once enterprise/agency demand appears; DPA template for B2B customers.
