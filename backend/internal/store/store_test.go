package store_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/testutil"
	"github.com/mbhat24/ugc-ad-generator/backend/migrations"
)

var ctx = context.Background()

func noop(context.Context, pgx.Tx, string) error { return nil }

func newAds(n int) []store.NewAd {
	out := make([]store.NewAd, n)
	for i := range out {
		out[i] = store.NewAd{Hook: "h", Script: "s", Caption: "c", Avatar: "a", Hashtags: []string{"#x"}}
	}
	return out
}

func TestMigrateIsIdempotent(t *testing.T) {
	s := testutil.Store(t)
	require.NoError(t, s.Migrate(ctx, migrations.FS))
	require.NoError(t, s.Migrate(ctx, migrations.FS))
}

func TestEnsureProfileGrantsSignupCreditOnce(t *testing.T) {
	s := testutil.Store(t)
	id, email := testutil.User(t, s)
	for range 3 {
		_, err := s.EnsureProfile(ctx, id, email, 1)
		require.NoError(t, err)
	}
	bal, err := s.Balance(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 1, bal)
}

func TestConcurrentProjectCreationNeverOverdraws(t *testing.T) {
	s := testutil.Store(t)
	id, _ := testutil.User(t, s) // 1 credit
	var ok, denied atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.CreateProject(ctx, id, "https://x.test", store.Product{Title: "p", Images: []string{}}, newAds(1), noop)
			switch {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, store.ErrInsufficientCredits):
				denied.Add(1)
			default:
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, ok.Load())
	require.EqualValues(t, 11, denied.Load())
	bal, _ := s.Balance(ctx, id)
	require.Equal(t, 0, bal)
}

func TestCreateProjectRollsBackWhenEnqueueFails(t *testing.T) {
	s := testutil.Store(t)
	id, _ := testutil.User(t, s)
	boom := errors.New("queue down")
	_, err := s.CreateProject(ctx, id, "https://x.test", store.Product{Title: "p", Images: []string{}}, newAds(1), func(context.Context, pgx.Tx, string) error { return boom })
	require.ErrorIs(t, err, boom)
	bal, _ := s.Balance(ctx, id)
	require.Equal(t, 1, bal, "credit must not be spent when the job cannot be enqueued")
	ps, _ := s.ListProjects(ctx, id, 10)
	require.Empty(t, ps)
}

func TestFailAdRefundsExactlyOnce(t *testing.T) {
	s := testutil.Store(t)
	id, _ := testutil.User(t, s)
	pid, err := s.CreateProject(ctx, id, "https://x.test", store.Product{Title: "p", Images: []string{}}, newAds(1), noop)
	require.NoError(t, err)
	p, err := s.GetProject(ctx, pid, id)
	require.NoError(t, err)
	adID := p.Ads[0].ID

	var wg sync.WaitGroup
	var changed atomic.Int32
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := s.FailAd(ctx, adID, id, "boom")
			require.NoError(t, err)
			if c {
				changed.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, changed.Load())
	bal, _ := s.Balance(ctx, id)
	require.Equal(t, 1, bal)
	done, _ := s.CompleteAd(ctx, adID, "u", "")
	require.False(t, done, "a failed ad must not complete")
}

func TestProjectsAreIsolatedPerUser(t *testing.T) {
	s := testutil.Store(t)
	a, _ := testutil.User(t, s)
	b, _ := testutil.User(t, s)
	pid, err := s.CreateProject(ctx, a, "https://x.test", store.Product{Title: "p", Images: []string{}}, newAds(1), noop)
	require.NoError(t, err)
	_, err = s.GetProject(ctx, pid, b)
	require.ErrorIs(t, err, store.ErrNotFound)
	_, err = s.GetProject(ctx, "not-a-uuid", a)
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestRateLimit(t *testing.T) {
	s := testutil.Store(t)
	key := "t:" + time.Now().Format(time.RFC3339Nano)
	var got []bool
	for range 4 {
		over, err := s.RateLimitHit(ctx, key, 3, 60)
		require.NoError(t, err)
		got = append(got, over)
	}
	require.Equal(t, []bool{false, false, false, true}, got)
}

func TestStaleAdsAndStripeDedupe(t *testing.T) {
	s := testutil.Store(t)
	id, _ := testutil.User(t, s)
	pid, _ := s.CreateProject(ctx, id, "https://x.test", store.Product{Title: "p", Images: []string{}}, newAds(1), noop)
	p, _ := s.GetProject(ctx, pid, id)
	_, err := s.Pool.Exec(ctx, "update ads set updated_at = now() - interval '10 minutes' where id::text = $1", p.Ads[0].ID)
	require.NoError(t, err)
	ids, err := s.StaleAds(ctx, 3*time.Minute, 100)
	require.NoError(t, err)
	require.Contains(t, ids, p.Ads[0].ID)

	fresh, _ := s.RecordStripeEvent(ctx, "evt_"+pid, "x")
	again, _ := s.RecordStripeEvent(ctx, "evt_"+pid, "x")
	require.True(t, fresh)
	require.False(t, again)
}
