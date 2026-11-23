-- Run once as the database owner. The web tier connects as adreel_web (no DDL, no pg-boss schema except enqueue).
-- The worker and migrations keep using the owner role.
create role adreel_web login password 'CHANGE_ME';
grant usage on schema public to adreel_web;
grant select, insert, update, delete on profiles, credit_ledger, projects, ads, stripe_events, rate_limits to adreel_web;
grant usage, select on all sequences in schema public to adreel_web;
grant select on schema_migrations to adreel_web;
-- pg-boss: the web tier only enqueues jobs
grant usage on schema pgboss to adreel_web;
grant select, insert, update on all tables in schema pgboss to adreel_web;
grant usage on all sequences in schema pgboss to adreel_web;
grant execute on all functions in schema pgboss to adreel_web;
-- RLS is enabled; the web role needs to bypass it for server-side access (it is not exposed to end users).
-- NOTE: granting BYPASSRLS needs a superuser. On Supabase (where your role is not a superuser) instead add permissive policies
-- `create policy web_all on <table> to adreel_web using (true) with check (true);` for each table above.
alter role adreel_web bypassrls;
