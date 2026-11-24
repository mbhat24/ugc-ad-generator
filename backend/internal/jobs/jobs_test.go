package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/testutil"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/video"
)

type fake struct {
	startID  string
	startErr error
	poll     video.Result
	pollErr  error
	cleaned  []string
}

func (f *fake) Name() string { return "fake" }
func (f *fake) Start(context.Context, store.Ad, store.Product) (string, error) {
	return f.startID, f.startErr
}
func (f *fake) Poll(context.Context, string) (video.Result, error) { return f.poll, f.pollErr }
func (f *fake) Cleanup(_ context.Context, id string)               { f.cleaned = append(f.cleaned, id) }

var ctx = context.Background()

func setup(t *testing.T, f *fake) (*GenerateWorker, *store.Store, string, string) {
	t.Helper()
	st := testutil.Store(t)
	uid, _ := testutil.User(t, st)
	pid, err := st.CreateProject(ctx, uid, "https://x.test", store.Product{Title: "p", Images: []string{}}, []store.NewAd{{Hook: "h", Script: "s", Caption: "c", Avatar: "a", Hashtags: []string{}}}, func(context.Context, pgx.Tx, string) error { return nil })
	require.NoError(t, err)
	p, _ := st.GetProject(ctx, pid, uid)
	w := &GenerateWorker{D: Deps{Store: st, Provider: f, Cfg: config.Config{AdMaxAge: 25 * time.Minute}}}
	return w, st, uid, p.Ads[0].ID
}

func run(w *GenerateWorker, adID string) error {
	return w.Work(ctx, &river.Job[GenerateAdArgs]{Args: GenerateAdArgs{AdID: adID}, JobRow: &rivertype.JobRow{}})
}

func snoozed(err error) (time.Duration, bool) {
	var s *river.JobSnoozeError
	if errors.As(err, &s) {
		return s.Duration, true
	}
	return 0, false
}

func status(t *testing.T, st *store.Store, uid, adID string) store.Ad {
	t.Helper()
	ps, err := st.ListProjects(ctx, uid, 50)
	require.NoError(t, err)
	for _, p := range ps {
		for _, a := range p.Ads {
			if a.ID == adID {
				return a
			}
		}
	}
	t.Fatal("ad not found")
	return store.Ad{}
}

func TestHappyPath(t *testing.T) {
	f := &fake{startID: "req_1"}
	w, st, uid, ad := setup(t, f)

	_, ok := snoozed(run(w, ad))
	require.True(t, ok, "after start the job re-polls later")
	require.Equal(t, store.AdProcessing, status(t, st, uid, ad).Status)

	f.poll = video.Result{State: video.Processing}
	_, ok = snoozed(run(w, ad))
	require.True(t, ok)

	f.poll = video.Result{State: video.Done, VideoURL: "https://cdn/v.mp4"}
	require.NoError(t, run(w, ad))
	a := status(t, st, uid, ad)
	require.Equal(t, store.AdDone, a.Status)
	require.Equal(t, "https://cdn/v.mp4", a.VideoURL)
	require.NoError(t, run(w, ad), "re-delivery of a finished ad is a no-op")
}

func TestProviderFailureRefunds(t *testing.T) {
	f := &fake{startID: "req_2"}
	w, st, uid, ad := setup(t, f)
	_, _ = snoozed(run(w, ad))
	f.poll = video.Result{State: video.Failed, Error: "moderation"}
	require.NoError(t, run(w, ad))
	require.NoError(t, run(w, ad))
	a := status(t, st, uid, ad)
	require.Equal(t, store.AdFailed, a.Status)
	require.Equal(t, "moderation", a.Error)
	bal, _ := st.Balance(ctx, uid)
	require.Equal(t, 1, bal)
}

func TestBusyProviderKeepsCreditAndRetries(t *testing.T) {
	f := &fake{startErr: &video.RetryLaterError{After: 42 * time.Second}}
	w, st, uid, ad := setup(t, f)
	d, ok := snoozed(run(w, ad))
	require.True(t, ok)
	require.Equal(t, 42*time.Second, d)
	require.Equal(t, store.AdPending, status(t, st, uid, ad).Status)
	bal, _ := st.Balance(ctx, uid)
	require.Equal(t, 0, bal)
}

func TestPermanentErrorFailsImmediately(t *testing.T) {
	f := &fake{startErr: &video.PermanentError{Msg: "bad model"}}
	w, st, uid, ad := setup(t, f)
	require.NoError(t, run(w, ad))
	require.Equal(t, store.AdFailed, status(t, st, uid, ad).Status)
	bal, _ := st.Balance(ctx, uid)
	require.Equal(t, 1, bal)
}

func TestTransientErrorIsRetriedByRiver(t *testing.T) {
	f := &fake{startErr: errors.New("connection reset")}
	w, st, uid, ad := setup(t, f)
	require.Error(t, run(w, ad))
	require.Equal(t, store.AdPending, status(t, st, uid, ad).Status)
}

func TestTimeoutFails(t *testing.T) {
	f := &fake{startID: "r"}
	w, st, uid, ad := setup(t, f)
	_, err := st.Pool.Exec(ctx, "update ads set created_at = now() - interval '2 hours' where id::text = $1", ad)
	require.NoError(t, err)
	require.NoError(t, run(w, ad))
	require.Equal(t, store.AdFailed, status(t, st, uid, ad).Status)
}

func TestErrorHandlerRefundsWhenRetriesExhausted(t *testing.T) {
	f := &fake{}
	_, st, uid, ad := setup(t, f)
	h := &ErrorHandler{Store: st}
	row := &rivertype.JobRow{Kind: "generate_ad", Attempt: 5, MaxAttempts: 5, EncodedArgs: []byte(`{"ad_id":"` + ad + `"}`)}
	h.HandleError(ctx, row, errors.New("x"))
	require.Equal(t, store.AdFailed, status(t, st, uid, ad).Status)
	bal, _ := st.Balance(ctx, uid)
	require.Equal(t, 1, bal)

	_, st2, uid2, ad2 := setup(t, f)
	(&ErrorHandler{Store: st2}).HandleError(ctx, &rivertype.JobRow{Kind: "generate_ad", Attempt: 2, MaxAttempts: 5, EncodedArgs: []byte(`{"ad_id":"` + ad2 + `"}`)}, errors.New("x"))
	require.Equal(t, store.AdPending, status(t, st2, uid2, ad2).Status, "not exhausted yet")
}
