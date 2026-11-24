// Package jobs defines the background work: video generation state machine and the reconciler.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/captions"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/storage"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/video"
)

const (
	pollDelay      = 8 * time.Second
	staleAfter     = 3 * time.Minute
	attemptTimeout = 4 * time.Minute
)

var adsFinished = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "adreel_ads_finished_total", Help: "Ads reaching a terminal state"}, []string{"status"})

func init() { prometheus.MustRegister(adsFinished) }

type GenerateAdArgs struct {
	AdID string `json:"ad_id" river:"unique"`
}

func (GenerateAdArgs) Kind() string { return "generate_ad" }

func (GenerateAdArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: 5,
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByState: []rivertype.JobState{rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateRetryable, rivertype.JobStateScheduled},
		},
	}
}

type ReconcileArgs struct{}

func (ReconcileArgs) Kind() string { return "reconcile" }

// EnqueueTx inserts a generation job inside the caller's transaction.
func EnqueueTx(c *river.Client[pgx.Tx]) store.EnqueueFunc {
	return func(ctx context.Context, tx pgx.Tx, adID string) error {
		_, err := c.InsertTx(ctx, tx, GenerateAdArgs{AdID: adID}, nil)
		return err
	}
}

type Deps struct {
	Store    *store.Store
	Provider video.Provider
	Storage  *storage.Store
	Cfg      config.Config
	HTTP     *http.Client
}

type GenerateWorker struct {
	river.WorkerDefaults[GenerateAdArgs]
	D Deps
}

func (w *GenerateWorker) Timeout(*river.Job[GenerateAdArgs]) time.Duration { return attemptTimeout }

func (w *GenerateWorker) fail(ctx context.Context, ad *store.AdWork, msg string) error {
	changed, err := w.D.Store.FailAd(ctx, ad.ID, ad.UserID, msg)
	if err != nil {
		return err
	}
	if changed {
		adsFinished.WithLabelValues("failed").Inc()
		slog.Warn("ad failed", "ad_id", ad.ID, "reason", msg)
	}
	return nil
}

func (w *GenerateWorker) Work(ctx context.Context, job *river.Job[GenerateAdArgs]) error {
	d := w.D
	ad, err := d.Store.LoadAdWork(ctx, job.Args.AdID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && !ad.Status.Active()) {
		return nil
	}
	if err != nil {
		return err
	}
	if time.Since(ad.CreatedAt) > d.Cfg.AdMaxAge {
		return w.fail(ctx, ad, "Video generation timed out")
	}

	if ad.VideoJobID == "" {
		id, err := d.Provider.Start(ctx, ad.Ad, ad.Product)
		var retry *video.RetryLaterError
		var perm *video.PermanentError
		switch {
		case errors.As(err, &retry):
			_ = d.Store.TouchAd(ctx, ad.ID)
			return river.JobSnooze(retry.After)
		case errors.As(err, &perm):
			return w.fail(ctx, ad, perm.Msg)
		case err != nil:
			return fmt.Errorf("start generation: %w", err)
		}
		if err := d.Store.SetProcessing(ctx, ad.ID, id); err != nil {
			return err
		}
		return river.JobSnooze(pollDelay)
	}

	res, err := d.Provider.Poll(ctx, ad.VideoJobID)
	if err != nil {
		slog.Warn("poll failed, will retry", "ad_id", ad.ID, "err", err)
		_ = d.Store.TouchAd(ctx, ad.ID)
		return river.JobSnooze(pollDelay)
	}
	switch res.State {
	case video.Processing:
		_ = d.Store.TouchAd(ctx, ad.ID)
		return river.JobSnooze(pollDelay)
	case video.Failed:
		return w.fail(ctx, ad, res.Error)
	}
	return w.finalize(ctx, ad, res.VideoURL)
}

func (w *GenerateWorker) finalize(ctx context.Context, ad *store.AdWork, sourceURL string) error {
	d := w.D
	url, key, stored := sourceURL, "", false
	if d.Storage != nil {
		k, u, err := w.rehost(ctx, ad, sourceURL)
		if err != nil {
			slog.Warn("storage failed, serving provider URL", "ad_id", ad.ID, "err", err)
		} else {
			key, url, stored = k, u, true
		}
	}
	ok, err := d.Store.CompleteAd(ctx, ad.ID, url, key)
	if err != nil {
		return err
	}
	if ok {
		adsFinished.WithLabelValues("done").Inc()
		slog.Info("ad done", "ad_id", ad.ID, "stored", stored)
	}
	if stored {
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		d.Provider.Cleanup(cctx, ad.VideoJobID)
	}
	return nil
}

func (w *GenerateWorker) rehost(ctx context.Context, ad *store.AdWork, sourceURL string) (key, url string, err error) {
	dir, err := os.MkdirTemp("", "ad-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(dir)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	resp, err := w.D.HTTP.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}
	in, err := os.Create(filepath.Join(dir, "in.mp4"))
	if err != nil {
		return "", "", err
	}
	if _, err := io.Copy(in, io.LimitReader(resp.Body, 500<<20)); err != nil {
		in.Close()
		return "", "", err
	}
	in.Close()
	file := filepath.Join(dir, "in.mp4")
	if err := captions.Burn(ctx, dir, ad.Hook+". "+ad.Script); err != nil {
		slog.Warn("captions skipped", "ad_id", ad.ID, "err", err)
	} else {
		file = filepath.Join(dir, "out.mp4")
	}
	return w.D.Storage.Upload(ctx, file, fmt.Sprintf("videos/%s/%s.mp4", ad.UserID, ad.ID))
}

// ReconcileWorker re-enqueues active ads that have made no progress, and prunes expired rate-limit rows.
type ReconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	Store *store.Store
}

func (w *ReconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileArgs]) error {
	c := river.ClientFromContext[pgx.Tx](ctx)
	ids, err := w.Store.StaleAds(ctx, staleAfter, 500)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := c.Insert(ctx, GenerateAdArgs{AdID: id}, nil); err != nil {
			return err
		}
	}
	if len(ids) > 0 {
		slog.Warn("reconciler re-enqueued stale ads", "count", len(ids))
	}
	_, err = w.Store.CleanupRateLimits(ctx)
	return err
}

// ErrorHandler fails and refunds an ad once its job is out of retries (or panics).
type ErrorHandler struct{ Store *store.Store }

func (h *ErrorHandler) give(ctx context.Context, job *rivertype.JobRow, msg string) {
	if job.Kind != (GenerateAdArgs{}).Kind() || job.Attempt < job.MaxAttempts {
		return
	}
	var a GenerateAdArgs
	if json.Unmarshal(job.EncodedArgs, &a) != nil {
		return
	}
	if w, err := h.Store.LoadAdWork(ctx, a.AdID); err == nil {
		if changed, _ := h.Store.FailAd(ctx, w.ID, w.UserID, "Video generation failed after retries"); changed {
			adsFinished.WithLabelValues("failed").Inc()
		}
	}
	slog.Error("job discarded", "kind", job.Kind, "job_id", job.ID, "err", msg)
}

func (h *ErrorHandler) HandleError(ctx context.Context, job *rivertype.JobRow, err error) *river.ErrorHandlerResult {
	slog.Warn("job error", "kind", job.Kind, "job_id", job.ID, "attempt", job.Attempt, "err", err)
	h.give(ctx, job, err.Error())
	return nil
}

func (h *ErrorHandler) HandlePanic(ctx context.Context, job *rivertype.JobRow, v any, trace string) *river.ErrorHandlerResult {
	slog.Error("job panic", "kind", job.Kind, "job_id", job.ID, "panic", fmt.Sprint(v), "trace", trace)
	h.give(ctx, job, fmt.Sprint(v))
	return nil
}
