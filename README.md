# AdReel - UGC video ads from a product URL

Next.js app + background worker. Supabase (Auth + Postgres), Stripe subscriptions, S3-compatible storage, Higgsfield video generation.

## Architecture
- `web`: Next.js (stateless, scale horizontally). Creates projects, charges credits, enqueues jobs.
- `worker`: pg-boss queue consumer. Starts/polls Higgsfield, burns captions with ffmpeg, uploads to S3/R2, refunds credits on failure.
- Postgres: users' profiles, projects, ads, credit ledger (balance = sum of ledger), job queue.

## Setup
1. Create a Supabase project. Copy URL, anon key and the Postgres connection string (pooler, session mode) into `.env.local` (see `.env.example`).
2. `npm run migrate`
3. Stripe: create 3 recurring prices, set `STRIPE_PRICE_*`, point a webhook at `/api/stripe/webhook` (events: `customer.subscription.*`, `invoice.paid`).
4. Storage: create an R2/S3 bucket with a public URL and set `S3_*`.
5. Run: `npm run dev` and `npm run worker` in two terminals.

## Local development (no accounts needed)
```
docker compose -f docker-compose.dev.yml up -d   # Postgres
printf 'DATABASE_URL=postgres://adreel:adreel@localhost:5432/adreel\nAPP_URL=http://localhost:3000\n' > .env.local
npm run migrate
npm run dev        # terminal 1
npm run worker     # terminal 2
```
Without `NEXT_PUBLIC_SUPABASE_URL` the app uses a dev-only login (any email, password ignored, disabled when `NODE_ENV=production`), the mock video provider, and template copy. Set the real keys to switch each integration on.

## Tests
`npm test` (needs the dev Postgres; the concurrency test also needs `npm run dev` running and is skipped otherwise).

## Docs
- [Documentation index](docs/README.md)
- [Architecture](docs/architecture/README.md)
- [Roadmap](docs/roadmap.md)

## Docker
`docker compose up --build` (reads `.env`; `NEXT_PUBLIC_SUPABASE_*` are baked in at build time). Scale workers with `docker compose up --scale worker=N`.
