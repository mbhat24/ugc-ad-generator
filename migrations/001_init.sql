create table if not exists profiles (
  id uuid primary key,
  email text not null,
  plan text not null default 'free',
  stripe_customer_id text unique,
  created_at timestamptz not null default now()
);
create table if not exists credit_ledger (
  id bigserial primary key,
  user_id uuid not null references profiles(id) on delete cascade,
  delta int not null,
  reason text not null,
  ref text,
  created_at timestamptz not null default now()
);
create index if not exists credit_ledger_user_idx on credit_ledger(user_id);
create unique index if not exists credit_ledger_ref_idx on credit_ledger(reason, ref) where ref is not null;
create table if not exists projects (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references profiles(id) on delete cascade,
  url text not null,
  product jsonb not null,
  created_at timestamptz not null default now()
);
create index if not exists projects_user_idx on projects(user_id, created_at desc);
create table if not exists ads (
  id uuid primary key default gen_random_uuid(),
  project_id uuid not null references projects(id) on delete cascade,
  position int not null,
  hook text not null,
  script text not null,
  caption text not null,
  hashtags jsonb not null default '[]',
  avatar text not null,
  status text not null default 'pending',
  video_job_id text,
  video_url text,
  error text,
  created_at timestamptz not null default now()
);
create index if not exists ads_project_idx on ads(project_id);
