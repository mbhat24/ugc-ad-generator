alter table ads add column if not exists updated_at timestamptz not null default now();
alter table ads add constraint ads_status_chk check (status in ('pending','processing','done','failed'));
create index if not exists ads_active_idx on ads(updated_at) where status in ('pending','processing');
create index if not exists ads_job_idx on ads(video_job_id) where video_job_id is not null;

alter table profiles add constraint profiles_plan_chk check (plan in ('free','starter','growth','scale'));

-- ledger outlives the user for financial records; user_id is kept as an opaque id
alter table credit_ledger drop constraint if exists credit_ledger_user_id_fkey;

create table if not exists stripe_events (
  id text primary key,
  type text not null,
  received_at timestamptz not null default now()
);

create table if not exists rate_limits (
  key text not null,
  window_start timestamptz not null,
  count int not null default 0,
  primary key (key, window_start)
);
