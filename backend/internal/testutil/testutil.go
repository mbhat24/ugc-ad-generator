// Package testutil provides shared helpers for integration tests against a real Postgres.
package testutil

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
	"github.com/mbhat24/ugc-ad-generator/backend/migrations"
)

var (
	once sync.Once
	st   *store.Store
	err  error
)

// Store returns a migrated store, or skips the test when no database is configured.
func Store(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run database tests")
	}
	once.Do(func() {
		st, err = store.Open(context.Background(), store.Options{URL: url, SSLMode: "off", MaxConn: 20, AppName: "adreel-test"})
		if err == nil {
			err = st.Migrate(context.Background(), migrations.FS)
		}
	})
	if err != nil {
		t.Fatalf("test database: %v", err)
	}
	return st
}

// User creates a profile (with the signup credit) and returns its id and email.
func User(t *testing.T, s *store.Store) (id, email string) {
	t.Helper()
	id = uuid.NewString()
	email = id[:8] + "@test.dev"
	if _, err := s.EnsureProfile(context.Background(), id, email, 1); err != nil {
		t.Fatal(err)
	}
	return id, email
}
