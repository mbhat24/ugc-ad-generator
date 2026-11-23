alter table ads add column if not exists video_key text;

-- Supabase exposes the public schema through PostgREST with the anon key. With RLS enabled and no
-- policies, anon/authenticated get no access; the application connects as a privileged role.
alter table profiles enable row level security;
alter table credit_ledger enable row level security;
alter table projects enable row level security;
alter table ads enable row level security;
alter table stripe_events enable row level security;
alter table rate_limits enable row level security;
alter table schema_migrations enable row level security;

do $$
declare r text;
begin
  foreach r in array array['anon','authenticated'] loop
    if exists (select 1 from pg_roles where rolname = r) then
      execute format('revoke all on all tables in schema public from %I', r);
      execute format('revoke all on all sequences in schema public from %I', r);
      execute format('alter default privileges in schema public revoke all on tables from %I', r);
    end if;
  end loop;
end $$;
