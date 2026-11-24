// Package store is the Postgres data layer: connection pool, transactions, migrations and queries.
package store

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrInsufficientCredits = errors.New("insufficient credits")
)

type Store struct{ Pool *pgxpool.Pool }

type Options struct {
	URL     string
	SSLMode string // off | require | verify
	CA      string
	MaxConn int32
	AppName string
}

func Open(ctx context.Context, o Options) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(o.URL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	switch o.SSLMode {
	case "off":
		cfg.ConnConfig.TLSConfig = nil
	case "require", "verify":
		tc := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: cfg.ConnConfig.Host, InsecureSkipVerify: o.SSLMode == "require"} //nolint:gosec // "require" matches libpq semantics: encrypt without verifying
		if o.CA != "" {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM([]byte(o.CA)) {
				return nil, errors.New("DATABASE_CA is not valid PEM")
			}
			tc.RootCAs = pool
		}
		cfg.ConnConfig.TLSConfig = tc
	}
	if o.MaxConn > 0 {
		cfg.MaxConns = o.MaxConn
	}
	cfg.MaxConnIdleTime = 30 * time.Second
	cfg.HealthCheckPeriod = 30 * time.Second
	cfg.ConnConfig.RuntimeParams["application_name"] = o.AppName
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "15000"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{Pool: pool}, nil
}

func (s *Store) Close() { s.Pool.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.Pool.Ping(ctx) }

// InTx runs fn in a transaction and commits if it returns nil.
func (s *Store) InTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
