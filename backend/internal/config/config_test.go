package config

import (
	"strings"
	"testing"
)

func TestProductionRequiresEverything(t *testing.T) {
	c := Config{Env: "production"}
	p := strings.Join(c.Problems(), "\n")
	for _, want := range []string{"DATABASE_URL", "SUPABASE_URL", "APP_URL", "Stripe", "S3"} {
		if !strings.Contains(p, want) {
			t.Errorf("missing problem for %s in:\n%s", want, p)
		}
	}
}

func TestPartialConfigIsRejected(t *testing.T) {
	c := Config{DatabaseURL: "x", StripeSecretKey: "sk", HiggsfieldKeyID: "id"}
	p := strings.Join(c.Problems(), "\n")
	if !strings.Contains(p, "STRIPE_WEBHOOK_SECRET") || !strings.Contains(p, "Higgsfield is partly configured") {
		t.Errorf("unexpected: %s", p)
	}
}

func TestDevDefaultsAreValid(t *testing.T) {
	if p := (Config{DatabaseURL: "x", S3Private: true}).Problems(); len(p) != 0 {
		t.Errorf("dev config should be valid: %v", p)
	}
}
