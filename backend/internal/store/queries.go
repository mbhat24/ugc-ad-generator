package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func nz(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// EnsureProfile upserts the profile and, atomically on first creation, grants the signup credit.
func (s *Store) EnsureProfile(ctx context.Context, id, email string, signupCredits int) (Profile, error) {
	var p Profile
	err := s.Pool.QueryRow(ctx, `
		with p as (
		  insert into profiles (id, email) values ($1::uuid, $2)
		  on conflict (id) do update set email = excluded.email
		  returning id, email, plan, stripe_customer_id, (xmax = 0) as created),
		g as (
		  insert into credit_ledger (user_id, delta, reason, ref)
		  select id, $3::int, 'signup', id::text from p where created
		  on conflict do nothing)
		select id::text, email, plan, stripe_customer_id from p`, id, email, signupCredits).
		Scan(&p.ID, &p.Email, &p.Plan, &p.StripeCustomerID)
	return p, err
}

func (s *Store) Balance(ctx context.Context, userID string) (int, error) {
	var b int
	err := s.Pool.QueryRow(ctx, "select coalesce(sum(delta), 0)::int from credit_ledger where user_id = $1", userID).Scan(&b)
	return b, err
}

func (s *Store) Usage(ctx context.Context, userID string) (hourlyProjects, dailyAds int, err error) {
	err = s.Pool.QueryRow(ctx, `
		select count(distinct p.id) filter (where p.created_at > now() - interval '1 hour')::int,
		       count(a.id) filter (where p.created_at > now() - interval '1 day')::int
		from projects p left join ads a on a.project_id = p.id where p.user_id = $1`, userID).Scan(&hourlyProjects, &dailyAds)
	return
}

// EnqueueFunc enqueues background work inside the caller's transaction (transactional outbox).
type EnqueueFunc func(ctx context.Context, tx pgx.Tx, adID string) error

func (s *Store) CreateProject(ctx context.Context, userID, url string, product Product, ads []NewAd, enqueue EnqueueFunc) (string, error) {
	var pid string
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "select 1 from profiles where id = $1 for update", userID); err != nil {
			return err
		}
		var bal int
		if err := tx.QueryRow(ctx, "select coalesce(sum(delta), 0)::int from credit_ledger where user_id = $1", userID).Scan(&bal); err != nil {
			return err
		}
		if bal < len(ads) {
			return ErrInsufficientCredits
		}
		if err := tx.QueryRow(ctx, "insert into projects (user_id, url, product) values ($1, $2, $3) returning id::text", userID, url, product).Scan(&pid); err != nil {
			return err
		}
		for i, a := range ads {
			var adID string
			if err := tx.QueryRow(ctx, `insert into ads (project_id, position, hook, script, caption, hashtags, avatar)
				values ($1, $2, $3, $4, $5, $6, $7) returning id::text`, pid, i, a.Hook, a.Script, a.Caption, a.Hashtags, a.Avatar).Scan(&adID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, "insert into credit_ledger (user_id, delta, reason, ref) values ($1, -1, 'ad', $2)", userID, adID); err != nil {
				return err
			}
			if err := enqueue(ctx, tx, adID); err != nil {
				return err
			}
		}
		return nil
	})
	return pid, err
}

const adCols = `a.id::text, a.hook, a.script, a.caption, a.hashtags, a.avatar, a.status, a.video_job_id, a.video_url, a.video_key, a.error, a.updated_at, a.created_at`

type scanner interface{ Scan(dest ...any) error }

func scanAd(r scanner) (Ad, error) {
	var a Ad
	var job, url, key, errMsg *string
	err := r.Scan(&a.ID, &a.Hook, &a.Script, &a.Caption, &a.Hashtags, &a.Avatar, &a.Status, &job, &url, &key, &errMsg, &a.UpdatedAt, &a.CreatedAt)
	a.VideoJobID, a.VideoURL, a.VideoKey, a.Error = nz(job), nz(url), nz(key), nz(errMsg)
	if a.Hashtags == nil {
		a.Hashtags = []string{}
	}
	return a, err
}

func (s *Store) loadAds(ctx context.Context, projects []Project) error {
	if len(projects) == 0 {
		return nil
	}
	ids := make([]string, len(projects))
	idx := map[string]int{}
	for i, p := range projects {
		ids[i], idx[p.ID] = p.ID, i
	}
	rows, err := s.Pool.Query(ctx, "select a.project_id::text, "+adCols+" from ads a where a.project_id = any($1::uuid[]) order by a.position", ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var pid string
		var a Ad
		var job, url, key, errMsg *string
		if err := rows.Scan(&pid, &a.ID, &a.Hook, &a.Script, &a.Caption, &a.Hashtags, &a.Avatar, &a.Status, &job, &url, &key, &errMsg, &a.UpdatedAt, &a.CreatedAt); err != nil {
			return err
		}
		a.VideoJobID, a.VideoURL, a.VideoKey, a.Error = nz(job), nz(url), nz(key), nz(errMsg)
		if a.Hashtags == nil {
			a.Hashtags = []string{}
		}
		projects[idx[pid]].Ads = append(projects[idx[pid]].Ads, a)
	}
	return rows.Err()
}

func (s *Store) ListProjects(ctx context.Context, userID string, limit int) ([]Project, error) {
	rows, err := s.Pool.Query(ctx, "select id::text, url, product, created_at from projects where user_id = $1 order by created_at desc limit $2", userID, limit)
	if err != nil {
		return nil, err
	}
	out := []Project{}
	for rows.Next() {
		p := Project{UserID: userID, Ads: []Ad{}}
		if err := rows.Scan(&p.ID, &p.URL, &p.Product, &p.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, s.loadAds(ctx, out)
}

func (s *Store) GetProject(ctx context.Context, id, userID string) (*Project, error) {
	p := Project{UserID: userID, Ads: []Ad{}}
	err := s.Pool.QueryRow(ctx, "select id::text, url, product, created_at from projects where id::text = $1 and user_id = $2", id, userID).Scan(&p.ID, &p.URL, &p.Product, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	ps := []Project{p}
	if err := s.loadAds(ctx, ps); err != nil {
		return nil, err
	}
	return &ps[0], nil
}

type AdWork struct {
	Ad
	UserID  string
	Product Product
}

func (s *Store) LoadAdWork(ctx context.Context, adID string) (*AdWork, error) {
	w := &AdWork{}
	row := s.Pool.QueryRow(ctx, "select "+adCols+", p.user_id::text, p.product from ads a join projects p on p.id = a.project_id where a.id::text = $1", adID)
	var a Ad
	var job, url, key, errMsg *string
	err := row.Scan(&a.ID, &a.Hook, &a.Script, &a.Caption, &a.Hashtags, &a.Avatar, &a.Status, &job, &url, &key, &errMsg, &a.UpdatedAt, &a.CreatedAt, &w.UserID, &w.Product)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.VideoJobID, a.VideoURL, a.VideoKey, a.Error = nz(job), nz(url), nz(key), nz(errMsg)
	w.Ad = a
	return w, nil
}

func (s *Store) SetProcessing(ctx context.Context, adID, jobID string) error {
	_, err := s.Pool.Exec(ctx, "update ads set status = 'processing', video_job_id = $2, updated_at = now() where id::text = $1 and status in ('pending','processing')", adID, jobID)
	return err
}

func (s *Store) TouchAd(ctx context.Context, adID string) error {
	_, err := s.Pool.Exec(ctx, "update ads set updated_at = now() where id::text = $1", adID)
	return err
}

func (s *Store) CompleteAd(ctx context.Context, adID, videoURL, videoKey string) (bool, error) {
	ct, err := s.Pool.Exec(ctx, "update ads set status = 'done', video_url = nullif($2,''), video_key = nullif($3,''), error = null, updated_at = now() where id::text = $1 and status = 'processing'", adID, videoURL, videoKey)
	return ct.RowsAffected() > 0, err
}

// FailAd marks an active ad failed and refunds its credit exactly once.
func (s *Store) FailAd(ctx context.Context, adID, userID, msg string) (bool, error) {
	if len(msg) > 300 {
		msg = msg[:300]
	}
	changed := false
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx, "update ads set status = 'failed', error = $2, updated_at = now() where id::text = $1 and status in ('pending','processing')", adID, msg)
		if err != nil || ct.RowsAffected() == 0 {
			return err
		}
		changed = true
		_, err = tx.Exec(ctx, "insert into credit_ledger (user_id, delta, reason, ref) values ($1, 1, 'refund', $2) on conflict do nothing", userID, adID)
		return err
	})
	return changed, err
}

func (s *Store) StaleAds(ctx context.Context, olderThan time.Duration, limit int) ([]string, error) {
	rows, err := s.Pool.Query(ctx, "select id::text from ads where status in ('pending','processing') and updated_at < now() - $1::interval order by updated_at limit $2", olderThan.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) ActiveAdsByVideoJob(ctx context.Context, jobID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, "select id::text from ads where video_job_id = $1 and status = 'processing'", jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// RateLimitHit increments a fixed-window counter and reports whether the limit is exceeded.
func (s *Store) RateLimitHit(ctx context.Context, key string, max, windowSec int) (bool, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		insert into rate_limits (key, window_start, count)
		values ($1, to_timestamp(floor(extract(epoch from now()) / $2::int) * $2::int), 1)
		on conflict (key, window_start) do update set count = rate_limits.count + 1
		returning count`, key, windowSec).Scan(&n)
	return n > max, err
}

func (s *Store) CleanupRateLimits(ctx context.Context) (int64, error) {
	ct, err := s.Pool.Exec(ctx, "delete from rate_limits where window_start < now() - interval '2 hours'")
	return ct.RowsAffected(), err
}

// ---- billing ----

func (s *Store) RecordStripeEvent(ctx context.Context, id, typ string) (bool, error) {
	ct, err := s.Pool.Exec(ctx, "insert into stripe_events (id, type) values ($1, $2) on conflict do nothing", id, typ)
	return ct.RowsAffected() > 0, err
}

func (s *Store) ForgetStripeEvent(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, "delete from stripe_events where id = $1", id)
	return err
}

func (s *Store) SetPlanByCustomer(ctx context.Context, customerID string, plan Plan) error {
	_, err := s.Pool.Exec(ctx, "update profiles set plan = $2 where stripe_customer_id = $1", customerID, string(plan))
	return err
}

func (s *Store) GrantSubscriptionCredits(ctx context.Context, customerID string, credits int, ref string) error {
	_, err := s.Pool.Exec(ctx, `insert into credit_ledger (user_id, delta, reason, ref)
		select id, $2, 'subscription', $3 from profiles where stripe_customer_id = $1 on conflict do nothing`, customerID, credits, ref)
	return err
}

func (s *Store) StripeCustomer(ctx context.Context, userID string) (string, error) {
	var c *string
	err := s.Pool.QueryRow(ctx, "select stripe_customer_id from profiles where id = $1", userID).Scan(&c)
	return nz(c), err
}

func (s *Store) SetStripeCustomer(ctx context.Context, userID, customerID string) error {
	_, err := s.Pool.Exec(ctx, "update profiles set stripe_customer_id = $2 where id = $1", userID, customerID)
	return err
}

// ---- account ----

type LedgerEntry struct {
	Delta     int       `json:"delta"`
	Reason    string    `json:"reason"`
	Ref       string    `json:"ref,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Store) Ledger(ctx context.Context, userID string) ([]LedgerEntry, error) {
	rows, err := s.Pool.Query(ctx, "select delta, reason, coalesce(ref,''), created_at from credit_ledger where user_id = $1 order by id", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LedgerEntry{}
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.Delta, &e.Reason, &e.Ref, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) DeleteProfile(ctx context.Context, userID string) error {
	_, err := s.Pool.Exec(ctx, "delete from profiles where id = $1", userID)
	return err
}
