# AdReel

Turns a product URL into short-form UGC video ads (hook, script, captions and an AI-avatar video) for TikTok and Reels.

## Architecture
```
Browser -> web (Next.js: UI + thin BFF proxy) -> api (Go) -> Postgres (Supabase)
                                                  |             ^
                                              River queue ----> worker (Go + ffmpeg) -> Higgsfield, S3/R2
```
| Directory | What |
|---|---|
| `backend/` | Go services: `cmd/api` (HTTP API), `cmd/worker` (background jobs), `cmd/migrate`. OpenAPI spec in `backend/api/openapi.yaml`. |
| `web/` | Next.js frontend. Holds the session cookie and proxies `/api/v1/*` to the API with the bearer token attached. No business logic. |
| `docs/` | [Architecture](docs/architecture/README.md), [launch checklist](docs/ops/launch-checklist.md), [roadmap](docs/roadmap.md). |
| `tools/` | Load test. |

Key properties: credits are an append-only ledger reserved in the same transaction that enqueues the job (River); failed ads are refunded exactly once; scraping validates the destination IP at connect time (SSRF/DNS-rebinding safe); videos live in a private bucket behind short-lived signed URLs; Stripe and Higgsfield webhooks are signature-verified and idempotent.

## Local development (no external accounts)
Requires Go 1.24+ (the toolchain auto-downloads), Node 22, Docker.
```
cp .env.example .env                                  # defaults work for local dev
docker compose -f docker-compose.dev.yml up -d        # Postgres
set -a; . ./.env; set +a
(cd backend && go run ./cmd/migrate)
(cd backend && go run ./cmd/api)                      # terminal 1  :8080
(cd backend && go run ./cmd/worker)                   # terminal 2  :9090 (needs ffmpeg for captions)
(cd web && npm ci && npm run dev)                     # terminal 3  :3000
```
Without `SUPABASE_URL` the API accepts a dev-only login (any email, no password; disabled when `APP_ENV=production`), the mock video provider and template copy are used. Set the real keys to switch each integration on.

## Tests
```
cd backend && DATABASE_URL=postgres://adreel:adreel@localhost:5432/adreel go test -race ./...
cd web && npm run lint && npm run build
node tools/loadtest.mjs        # BASE=http://localhost:8080 USERS=25
```
Database tests are skipped when no `DATABASE_URL`/`TEST_DATABASE_URL` is set. After changing `backend/api/openapi.yaml` run `cd web && npm run gen:api`.

## Deploy
`docker compose up --build` builds `migrate`, `api`, `worker` and `web` images from `.env` (see `.env.example`). Run `migrate` as a release step before rolling out. Scale `api`, `worker` and `web` independently. Go-live steps: [docs/ops/launch-checklist.md](docs/ops/launch-checklist.md).
