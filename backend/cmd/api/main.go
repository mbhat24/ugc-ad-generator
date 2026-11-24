// Command api serves the public HTTP API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/api"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/auth"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/billing"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/copywriter"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/logx"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/scrape"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/storage"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		slog.Error("api exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	logx.Setup(cfg.LogLevel, "api", cfg.ErrorWebhookURL)
	if err := cfg.Validate(); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, store.Options{URL: cfg.DatabaseURL, SSLMode: cfg.DatabaseSSL, CA: cfg.DatabaseCA, MaxConn: cfg.PoolMax, AppName: "adreel-api"})
	if err != nil {
		return err
	}
	defer st.Close()

	queue, err := river.NewClient(riverpgxv5.New(st.Pool), &river.Config{Logger: slog.Default()})
	if err != nil {
		return err
	}
	verifier, err := auth.NewVerifier(cfg)
	if err != nil {
		return err
	}
	stor, err := storage.New(ctx, cfg)
	if err != nil {
		return err
	}

	srv := &api.Server{
		Cfg: cfg, Store: st, Verifier: verifier, Queue: queue,
		Billing: billing.New(cfg, st), Storage: stor, Scraper: scrape.NewClient(),
		Writer: &copywriter.Writer{Key: cfg.OpenAIKey, Model: cfg.OpenAIModel},
		HTTP:   &http.Client{Timeout: 15 * time.Second},
	}
	hs := &http.Server{
		Addr: cfg.Addr, Handler: srv.Router(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 100 * time.Second, IdleTimeout: 90 * time.Second,
		MaxHeaderBytes: 32 << 10,
	}
	errc := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", cfg.Addr, "env", cfg.Env, "dev_auth", cfg.DevAuth)
		if err := hs.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	slog.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return hs.Shutdown(sctx)
}

// healthcheck lets minimal images (no curl/wget) probe readiness: `api healthcheck`.
func healthcheck() int {
	addr := config.Load().Addr
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get("http://" + addr + "/readyz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
