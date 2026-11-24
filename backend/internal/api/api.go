// Package api wires the HTTP surface of the service.
package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riverqueue/river"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/auth"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/billing"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/copywriter"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/httpx"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/jobs"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/moderation"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/scrape"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/storage"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
)

const signupCredits = 1

type Server struct {
	Cfg      config.Config
	Store    *store.Store
	Verifier *auth.Verifier
	Queue    *river.Client[pgx.Tx]
	Billing  *billing.Service
	Storage  *storage.Store
	Scraper  *scrape.Client
	Writer   *copywriter.Writer
	HTTP     *http.Client
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.Observe, httpx.SecurityHeaders(s.Cfg.Production()), httpx.CORS(s.Cfg.AllowedOrigins))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { httpx.JSON(w, 200, map[string]bool{"ok": true}) })
	r.Get("/readyz", s.ready)
	r.Get("/metrics", s.metrics(promhttp.Handler()))

	r.Route("/v1", func(r chi.Router) {
		r.Get("/plans", func(w http.ResponseWriter, _ *http.Request) { httpx.JSON(w, 200, store.Plans) })
		r.Post("/dev/login", s.devLogin)
		r.Post("/webhooks/stripe", s.stripeWebhook)
		r.Post("/webhooks/higgsfield", s.higgsfieldWebhook)

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(s.Verifier, s.Store, signupCredits))
			r.Get("/me", s.me)
			r.Get("/projects", s.listProjects)
			r.Post("/projects", s.createProject)
			r.Get("/projects/{id}", s.getProject)
			r.Post("/billing/checkout", s.checkout)
			r.Post("/billing/portal", s.portal)
			r.Get("/account/export", s.export)
			r.Delete("/account", s.deleteAccount)
		})
	})
	return r
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.Store.Ping(ctx); err != nil {
		httpx.JSON(w, http.StatusServiceUnavailable, map[string]bool{"ok": false})
		return
	}
	httpx.JSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) metrics(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if t := s.Cfg.MetricsToken; t != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+t)) != 1 {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "unauthenticated")
			return
		}
		h.ServeHTTP(w, r)
	}
}

// limited applies a fixed-window limit and writes a 429 when exceeded.
func (s *Server) limited(w http.ResponseWriter, r *http.Request, name string, max, windowSec int, extra string) bool {
	key := name + ":" + httpx.ClientIP(r, s.Cfg.TrustedProxyHops)
	if extra != "" {
		key += ":" + extra
	}
	over, err := s.Store.RateLimitHit(r.Context(), key, max, windowSec)
	if err != nil {
		slog.Error("rate limit", "err", err)
		return false
	}
	if over {
		w.Header().Set("Retry-After", strconv.Itoa(windowSec))
		httpx.Error(w, http.StatusTooManyRequests, "Too many requests. Try again later.", "rate_limited")
	}
	return over
}

func (s *Server) internal(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("request failed", "request_id", httpx.RequestID(r.Context()), "path", r.URL.Path, "err", err)
	httpx.Error(w, http.StatusInternalServerError, "Internal error", "internal")
}

// ---- auth-adjacent ----

func (s *Server) devLogin(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.DevAuth {
		http.NotFound(w, r)
		return
	}
	var in struct {
		Email string `json:"email"`
	}
	if httpx.DecodeJSON(r, &in, 4<<10) != nil {
		httpx.Error(w, 400, "Invalid request", "bad_request")
		return
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		httpx.Error(w, 400, "Enter a valid email", "bad_request")
		return
	}
	if s.limited(w, r, "devlogin", 60, 60, "") {
		return
	}
	tok, id, err := s.Verifier.DevToken(in.Email)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	httpx.JSON(w, 200, map[string]string{"token": tok, "id": id})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	httpx.JSON(w, 200, map[string]any{"id": u.ID, "email": u.Email, "plan": u.Plan, "balance": u.Balance, "hasBilling": u.StripeCustomerID != nil, "mock": s.Cfg.Production() == false && !s.Cfg.HiggsfieldEnabled()})
}

// ---- projects ----

func (s *Server) present(ctx context.Context, p *store.Project) {
	for i := range p.Ads {
		a := &p.Ads[i]
		if a.VideoKey != "" && s.Storage != nil {
			if u, err := s.Storage.SignedURL(ctx, a.VideoKey, time.Hour); err == nil {
				a.VideoURL = u
			} else {
				slog.Error("sign url", "err", err)
			}
		}
	}
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	ps, err := s.Store.ListProjects(r.Context(), u.ID, 50)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	httpx.JSON(w, 200, ps)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	p, err := s.Store.GetProject(r.Context(), chi.URLParam(r, "id"), u.ID)
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, 404, "Not found", "not_found")
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	s.present(r.Context(), p)
	httpx.JSON(w, 200, p)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	if s.limited(w, r, "projects", 30, 3600, u.ID) {
		return
	}
	var in struct {
		URL   string `json:"url"`
		Count int    `json:"count"`
	}
	if httpx.DecodeJSON(r, &in, 8<<10) != nil {
		httpx.Error(w, 400, "Invalid request", "bad_request")
		return
	}
	in.URL = strings.TrimSpace(in.URL)
	if len(in.URL) > 2000 || in.URL == "" {
		httpx.Error(w, 400, "Enter a valid product URL", "bad_request")
		return
	}
	count := min(max(in.Count, 1), 3)
	if u.Balance < count {
		httpx.JSON(w, 402, httpx.ErrorBody{Error: fmt.Sprintf("Not enough videos left (%d remaining). Upgrade your plan.", u.Balance), Code: "no_credits"})
		return
	}
	hourly, daily, err := s.Store.Usage(r.Context(), u.ID)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	if hourly >= s.Cfg.MaxProjectsPerHour {
		httpx.Error(w, 429, "Too many projects this hour. Try again later.", "rate_limited")
		return
	}
	if daily+count > s.Cfg.MaxAdsPerDay {
		httpx.Error(w, 429, "Daily generation limit reached. Try again tomorrow.", "rate_limited")
		return
	}

	product, err := s.Scraper.Product(r.Context(), in.URL)
	if err != nil {
		httpx.Error(w, 400, err.Error(), "bad_product_url")
		return
	}
	reason := moderation.CheckProduct(product)
	if reason == "" {
		reason = moderation.OpenAI(r.Context(), s.Cfg.OpenAIKey, product)
	}
	if reason != "" {
		slog.Warn("product blocked by moderation", "user_id", u.ID, "url", in.URL)
		httpx.Error(w, 422, reason, "moderation")
		return
	}
	ads := s.Writer.Generate(r.Context(), product, count)
	id, err := s.Store.CreateProject(r.Context(), u.ID, in.URL, product, ads, jobs.EnqueueTx(s.Queue))
	if errors.Is(err, store.ErrInsufficientCredits) {
		httpx.JSON(w, 402, httpx.ErrorBody{Error: "Not enough videos left. Upgrade your plan.", Code: "no_credits"})
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	slog.Info("project created", "user_id", u.ID, "project_id", id, "ads", len(ads))
	httpx.JSON(w, 201, map[string]string{"id": id})
}

// ---- billing ----

func (s *Server) checkout(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	if !s.Billing.Enabled() {
		httpx.Error(w, 501, "Billing is not configured.", "billing_disabled")
		return
	}
	if s.limited(w, r, "billing", 20, 3600, u.ID) {
		return
	}
	var in struct {
		Plan store.Plan `json:"plan"`
	}
	if httpx.DecodeJSON(r, &in, 1<<10) != nil || in.Plan == store.PlanFree {
		httpx.Error(w, 400, "Unknown plan", "bad_request")
		return
	}
	if _, ok := store.PlanByID(in.Plan); !ok {
		httpx.Error(w, 400, "Unknown plan", "bad_request")
		return
	}
	url, err := s.Billing.Checkout(r.Context(), u.ID, u.Email, in.Plan)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	httpx.JSON(w, 200, map[string]string{"url": url})
}

func (s *Server) portal(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	if !s.Billing.Enabled() {
		httpx.Error(w, 501, "Billing is not configured.", "billing_disabled")
		return
	}
	if s.limited(w, r, "billing", 20, 3600, u.ID) {
		return
	}
	url, err := s.Billing.Portal(r.Context(), u.ID)
	if err != nil {
		httpx.Error(w, 400, err.Error(), "bad_request")
		return
	}
	httpx.JSON(w, 200, map[string]string{"url": url})
}

func (s *Server) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.Error(w, 400, "bad body", "bad_request")
		return
	}
	switch err := s.Billing.HandleWebhook(r.Context(), body, r.Header.Get("Stripe-Signature")); {
	case err == nil:
		httpx.JSON(w, 200, map[string]bool{"received": true})
	case errors.Is(err, billing.ErrBadSignature):
		httpx.Error(w, 400, "Bad signature", "bad_signature")
	case errors.Is(err, billing.ErrDisabled):
		httpx.Error(w, 501, "Billing is not configured.", "billing_disabled")
	default:
		slog.Error("stripe webhook failed", "err", err)
		httpx.Error(w, 500, "Handler error", "internal")
	}
}

// ---- provider webhook (Standard Webhooks) ----

func verifyStandardWebhook(secret, id, ts string, body []byte, sigHeader string, now time.Time) bool {
	if secret == "" || id == "" || ts == "" {
		return false
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || now.Sub(time.Unix(sec, 0)).Abs() > 5*time.Minute {
		return false
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%s.%s.", id, ts)
	mac.Write(body)
	want := mac.Sum(nil)
	for _, part := range strings.Fields(sigHeader) {
		v, sig, ok := strings.Cut(part, ",")
		if !ok || v != "v1" {
			continue
		}
		if got, err := base64.StdEncoding.DecodeString(sig); err == nil && hmac.Equal(got, want) {
			return true
		}
	}
	return false
}

func (s *Server) higgsfieldWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || !verifyStandardWebhook(s.Cfg.HiggsfieldWebhookSecret, r.Header.Get("webhook-id"), r.Header.Get("webhook-timestamp"), body, r.Header.Get("webhook-signature"), time.Now()) {
		httpx.Error(w, 400, "Bad signature", "bad_signature")
		return
	}
	var ev struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(body, &ev); err != nil || ev.RequestID == "" {
		httpx.Error(w, 400, "Bad body", "bad_request")
		return
	}
	ids, err := s.Store.ActiveAdsByVideoJob(r.Context(), ev.RequestID)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	for _, id := range ids {
		if _, err := s.Queue.Insert(r.Context(), jobs.GenerateAdArgs{AdID: id}, nil); err != nil {
			s.internal(w, r, err)
			return
		}
	}
	httpx.JSON(w, 200, map[string]bool{"received": true})
}

// ---- account ----

func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	ps, err := s.Store.ListProjects(r.Context(), u.ID, 1000)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	for i := range ps {
		s.present(r.Context(), &ps[i])
	}
	ledger, err := s.Store.Ledger(r.Context(), u.ID)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="adreel-export.json"`)
	httpx.JSON(w, 200, map[string]any{"profile": map[string]any{"id": u.ID, "email": u.Email, "plan": u.Plan}, "projects": ps, "ledger": ledger})
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	var in struct {
		Confirm string `json:"confirm"`
	}
	if httpx.DecodeJSON(r, &in, 1<<10) != nil || !strings.EqualFold(in.Confirm, u.Email) {
		httpx.Error(w, 400, "Type your email to confirm account deletion", "bad_request")
		return
	}
	ctx := r.Context()
	if err := s.Billing.CancelAll(ctx, u.ID); err != nil {
		s.internal(w, r, fmt.Errorf("cancel subscriptions: %w", err))
		return
	}
	if s.Storage != nil {
		if err := s.Storage.DeletePrefix(ctx, "videos/"+u.ID+"/"); err != nil {
			s.internal(w, r, fmt.Errorf("delete media: %w", err))
			return
		}
	}
	if !s.Cfg.DevAuth {
		if s.Cfg.SupabaseServiceRoleKey == "" {
			httpx.Error(w, 501, "Account deletion is not configured", "not_configured")
			return
		}
		if err := s.deleteSupabaseUser(ctx, u.ID); err != nil {
			s.internal(w, r, fmt.Errorf("delete auth user: %w", err))
			return
		}
	}
	if err := s.Store.DeleteProfile(ctx, u.ID); err != nil {
		s.internal(w, r, err)
		return
	}
	slog.Info("account deleted", "user_id", u.ID)
	httpx.JSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) deleteSupabaseUser(ctx context.Context, id string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, s.Cfg.SupabaseURL+"/auth/v1/admin/users/"+id, nil)
	req.Header.Set("apikey", s.Cfg.SupabaseServiceRoleKey)
	req.Header.Set("Authorization", "Bearer "+s.Cfg.SupabaseServiceRoleKey)
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("supabase returned %d", resp.StatusCode)
	}
	return nil
}
