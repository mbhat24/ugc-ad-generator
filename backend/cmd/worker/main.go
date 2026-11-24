// Command worker processes background jobs (video generation, reconciliation).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/jobs"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/logx"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/storage"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/video"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	logx.Setup(cfg.LogLevel, "worker", cfg.ErrorWebhookURL)
	if err := cfg.Validate(); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, store.Options{URL: cfg.DatabaseURL, SSLMode: cfg.DatabaseSSL, CA: cfg.DatabaseCA, MaxConn: int32(cfg.WorkerConcurrency + 5), AppName: "adreel-worker"})
	if err != nil {
		return err
	}
	defer st.Close()
	stor, err := storage.New(ctx, cfg)
	if err != nil {
		return err
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, &jobs.GenerateWorker{D: jobs.Deps{Store: st, Provider: video.New(cfg), Storage: stor, Cfg: cfg, HTTP: &http.Client{Timeout: 3 * time.Minute}}})
	river.AddWorker(workers, &jobs.ReconcileWorker{Store: st})

	client, err := river.NewClient(riverpgxv5.New(st.Pool), &river.Config{
		Logger:       slog.Default(),
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: cfg.WorkerConcurrency}},
		Workers:      workers,
		ErrorHandler: &jobs.ErrorHandler{Store: st},
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(river.PeriodicInterval(time.Minute), func() (river.JobArgs, *river.InsertOpts) { return jobs.ReconcileArgs{}, nil }, &river.PeriodicJobOpts{RunOnStart: true}),
		},
	})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(`{"ok":true}`)) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if st.Ping(r.Context()) != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	})
	mux.Handle("/metrics", promhttp.Handler())
	hs := &http.Server{Addr: envOr("WORKER_ADDR", ":9090"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := hs.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			slog.Error("worker http", "err", err)
		}
	}()

	slog.Info("worker ready", "provider", video.New(cfg).Name(), "concurrency", cfg.WorkerConcurrency, "storage", stor != nil)
	<-ctx.Done()
	slog.Info("worker stopping")
	sctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := client.Stop(sctx); err != nil {
		slog.Warn("graceful stop timed out, cancelling jobs", "err", err)
		_ = client.StopAndCancel(context.Background())
	}
	_ = hs.Shutdown(sctx)
	return nil
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
