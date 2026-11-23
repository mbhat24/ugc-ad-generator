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

## Docs
- [Architecture](docs/ARCHITECTURE.md)
- [Roadmap and future work](docs/ROADMAP.md)

## Docker
`docker compose up --build` (reads `.env`; `NEXT_PUBLIC_SUPABASE_*` are baked in at build time). Scale workers with `docker compose up --scale worker=N`.
