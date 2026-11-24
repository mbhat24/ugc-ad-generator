package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
)

func sign(t *testing.T, secret string, method jwt.SigningMethod, c claims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, c).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDevTokenRoundTrip(t *testing.T) {
	v, _ := NewVerifier(config.Config{DevAuth: true, DevJWTSecret: "dev-secret-dev-secret-dev-secret!!"})
	tok, id, err := v.DevToken("A@Example.com")
	if err != nil {
		t.Fatal(err)
	}
	p, err := v.Verify(tok)
	if err != nil || p.ID != id || p.Email != "a@example.com" {
		t.Fatalf("verify: %+v %v", p, err)
	}
	if _, id2, _ := v.DevToken("a@example.com"); id2 != id {
		t.Error("dev ids must be deterministic per email")
	}
}

func TestDevAuthDisabledInProduction(t *testing.T) {
	v, _ := NewVerifier(config.Config{Env: "production", SupabaseJWTSecret: "s"})
	if _, _, err := v.DevToken("a@b.co"); err == nil {
		t.Fatal("dev tokens must not be mintable when dev auth is off")
	}
}

func TestSupabaseHS256(t *testing.T) {
	secret := "super-secret-jwt-token-with-at-least-32-characters"
	v, _ := NewVerifier(config.Config{Env: "production", SupabaseJWTSecret: secret})
	good := claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "u1", Audience: jwt.ClaimStrings{"authenticated"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}, Email: "x@y.co"}
	if p, err := v.Verify(sign(t, secret, jwt.SigningMethodHS256, good)); err != nil || p.ID != "u1" {
		t.Fatalf("good token rejected: %v", err)
	}
	bad := map[string]claims{
		"expired":   {RegisteredClaims: jwt.RegisteredClaims{Subject: "u1", Audience: jwt.ClaimStrings{"authenticated"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))}, Email: "x@y.co"},
		"wrong aud": {RegisteredClaims: jwt.RegisteredClaims{Subject: "u1", Audience: jwt.ClaimStrings{"anon"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}, Email: "x@y.co"},
		"no email":  {RegisteredClaims: jwt.RegisteredClaims{Subject: "u1", Audience: jwt.ClaimStrings{"authenticated"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}},
		"no exp":    {RegisteredClaims: jwt.RegisteredClaims{Subject: "u1", Audience: jwt.ClaimStrings{"authenticated"}}, Email: "x@y.co"},
	}
	for name, c := range bad {
		if _, err := v.Verify(sign(t, secret, jwt.SigningMethodHS256, c)); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
	if _, err := v.Verify(sign(t, "another-secret-another-secret-123456", jwt.SigningMethodHS256, good)); err == nil {
		t.Error("wrong signature accepted")
	}
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, good).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := v.Verify(none); err == nil {
		t.Error("alg=none accepted")
	}
}
