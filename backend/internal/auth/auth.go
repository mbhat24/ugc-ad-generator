// Package auth verifies Supabase-issued JWTs (JWKS or HS256) and, outside production, local dev tokens.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/httpx"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
)

type Principal struct {
	ID    string
	Email string
}

type Verifier struct {
	cfg  config.Config
	jwks keyfunc.Keyfunc
}

func NewVerifier(cfg config.Config) (*Verifier, error) {
	v := &Verifier{cfg: cfg}
	if cfg.SupabaseURL != "" {
		k, err := keyfunc.NewDefault([]string{cfg.SupabaseURL + "/auth/v1/.well-known/jwks.json"})
		if err != nil {
			return nil, fmt.Errorf("load JWKS: %w", err)
		}
		v.jwks = k
	}
	return v, nil
}

type claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (v *Verifier) keyfunc(t *jwt.Token) (any, error) {
	if v.cfg.DevAuth {
		if t.Method.Alg() != "HS256" {
			return nil, errors.New("unexpected alg")
		}
		return []byte(v.cfg.DevJWTSecret), nil
	}
	if t.Method.Alg() == "HS256" {
		if v.cfg.SupabaseJWTSecret == "" {
			return nil, errors.New("HS256 not configured")
		}
		return []byte(v.cfg.SupabaseJWTSecret), nil
	}
	if v.jwks == nil {
		return nil, errors.New("JWKS not configured")
	}
	return v.jwks.Keyfunc(t)
}

func (v *Verifier) Verify(token string) (*Principal, error) {
	var c claims
	opts := []jwt.ParserOption{jwt.WithValidMethods([]string{"HS256", "RS256", "ES256"}), jwt.WithExpirationRequired(), jwt.WithLeeway(30 * time.Second)}
	if !v.cfg.DevAuth {
		opts = append(opts, jwt.WithAudience("authenticated"))
	}
	t, err := jwt.ParseWithClaims(token, &c, v.keyfunc, opts...)
	if err != nil || !t.Valid {
		return nil, errors.New("invalid token")
	}
	if c.Subject == "" || c.Email == "" {
		return nil, errors.New("token missing subject or email")
	}
	return &Principal{ID: c.Subject, Email: strings.ToLower(c.Email)}, nil
}

// DevToken mints a token for local development. Only valid when DevAuth is enabled.
func (v *Verifier) DevToken(email string) (string, string, error) {
	if !v.cfg.DevAuth {
		return "", "", errors.New("dev auth disabled")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	id := uuid.NewSHA1(uuid.NameSpaceDNS, []byte(email)).String()
	c := claims{RegisteredClaims: jwt.RegisteredClaims{Subject: id, ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour))}, Email: email}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(v.cfg.DevJWTSecret))
	return s, id, err
}

type ctxKey int

const (
	userKey ctxKey = iota
)

type User struct {
	store.Profile
	Balance int
}

func UserFrom(ctx context.Context) *User { u, _ := ctx.Value(userKey).(*User); return u }

// Middleware authenticates the bearer token and loads (creating on first sight) the user's profile.
func Middleware(v *Verifier, st *store.Store, signupCredits int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || tok == "" {
				httpx.Error(w, http.StatusUnauthorized, "Not signed in", "unauthenticated")
				return
			}
			p, err := v.Verify(tok)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "Not signed in", "unauthenticated")
				return
			}
			prof, err := st.EnsureProfile(r.Context(), p.ID, p.Email, signupCredits)
			if err == nil {
				var bal int
				if bal, err = st.Balance(r.Context(), p.ID); err == nil {
					next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, &User{Profile: prof, Balance: bal})))
					return
				}
			}
			slog.Error("load user", "err", err)
			httpx.Error(w, http.StatusInternalServerError, "Internal error", "internal")
		})
	}
}
