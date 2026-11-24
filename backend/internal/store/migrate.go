package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"

	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

const migrationLock = 727001

// Migrate applies the embedded application migrations (recorded with checksums) and River's own schema.
func (s *Store) Migrate(ctx context.Context, files fs.FS) error {
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "select pg_advisory_lock($1)", migrationLock); err != nil {
		return err
	}
	defer func() { _, _ = conn.Exec(context.WithoutCancel(ctx), "select pg_advisory_unlock($1)", migrationLock) }()

	if _, err := conn.Exec(ctx, "create table if not exists schema_migrations (name text primary key, checksum text not null, applied_at timestamptz not null default now())"); err != nil {
		return err
	}
	rows, err := conn.Query(ctx, "select name, checksum from schema_migrations")
	if err != nil {
		return err
	}
	applied := map[string]string{}
	for rows.Next() {
		var n, c string
		if err := rows.Scan(&n, &c); err != nil {
			return err
		}
		applied[n] = c
	}
	rows.Close()

	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		b, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		checksum := hex.EncodeToString(sum[:])
		if prev, ok := applied[name]; ok {
			if prev != checksum {
				return fmt.Errorf("migration %s was modified after being applied", name)
			}
			continue
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(b)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, "insert into schema_migrations (name, checksum) values ($1, $2)", name, checksum); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		slog.Info("migration applied", "name", name)
	}

	m, err := rivermigrate.New(riverpgxv5.New(s.Pool), nil)
	if err != nil {
		return err
	}
	if _, err := m.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("river migrate: %w", err)
	}
	return nil
}
