// Command migrate applies application and queue schema migrations.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/logx"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
	"github.com/mbhat24/ugc-ad-generator/backend/migrations"
)

func main() {
	cfg := config.Load()
	logx.Setup(cfg.LogLevel, "migrate", "")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	st, err := store.Open(ctx, store.Options{URL: cfg.DatabaseURL, SSLMode: cfg.DatabaseSSL, CA: cfg.DatabaseCA, MaxConn: 2, AppName: "adreel-migrate"})
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer st.Close()
	if err := st.Migrate(ctx, migrations.FS); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
	slog.Info("migrations up to date")
}
