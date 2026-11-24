package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/stretchr/testify/require"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/auth"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/billing"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/copywriter"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/scrape"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/testutil"
)

type env struct {
	t     *testing.T
	srv   *httptest.Server
	shop  *httptest.Server
	s     *Server
	token string
}

func setup(t *testing.T) *env {
	t.Helper()
	st := testutil.Store(t)
	cfg := config.Config{Env: "development", DevAuth: true, DevJWTSecret: "dev-secret-dev-secret-dev-secret!!", MaxProjectsPerHour: 100, MaxAdsPerDay: 100, AllowedOrigins: []string{"http://app.test"}, HiggsfieldWebhookSecret: "whsec_" + base64.StdEncoding.EncodeToString([]byte("hook-secret"))}
	q, err := river.NewClient(riverpgxv5.New(st.Pool), &river.Config{})
	require.NoError(t, err)
	v, _ := auth.NewVerifier(cfg)
	shop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		title := "Merino Runner"
		if strings.Contains(r.URL.Path, "gun") {
			title = "Tactical rifle scope"
		}
		fmt.Fprintf(w, `<meta property="og:title" content="%s"><meta property="og:description" content="Soft and light">`, title)
	}))
	t.Cleanup(shop.Close)
	s := &Server{Cfg: cfg, Store: st, Verifier: v, Queue: q, Billing: billing.New(cfg, st), Scraper: scrape.NewClientForTest(), Writer: &copywriter.Writer{}, HTTP: http.DefaultClient}
	srv := httptest.NewServer(s.Router())
	t.Cleanup(srv.Close)
	tok, _, _ := v.DevToken(uuid.NewString()[:8] + "@test.dev")
	return &env{t: t, srv: srv, shop: shop, s: s, token: tok}
}

func (e *env) do(method, path, body string, hdr ...string) (int, map[string]any) {
	e.t.Helper()
	req, _ := http.NewRequest(method, e.srv.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if e.token != "" {
		req.Header.Set("Authorization", "Bearer "+e.token)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(e.t, err)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return resp.StatusCode, out
}

func TestAuthRequired(t *testing.T) {
	e := setup(t)
	e.token = ""
	for _, p := range []string{"/v1/me", "/v1/projects", "/v1/account/export"} {
		code, _ := e.do("GET", p, "")
		require.Equal(t, 401, code, p)
	}
	e.token = "garbage"
	code, _ := e.do("GET", "/v1/me", "")
	require.Equal(t, 401, code)
}

func TestHealthAndHeaders(t *testing.T) {
	e := setup(t)
	resp, err := http.Get(e.srv.URL + "/readyz")
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	require.NotEmpty(t, resp.Header.Get("X-Request-Id"))
}

func TestCreateProjectFlow(t *testing.T) {
	e := setup(t)
	code, me := e.do("GET", "/v1/me", "")
	require.Equal(t, 200, code)
	require.EqualValues(t, 1, me["balance"])

	code, out := e.do("POST", "/v1/projects", fmt.Sprintf(`{"url":%q,"count":1}`, e.shop.URL+"/p/shoe"))
	require.Equal(t, 201, code, out)
	id := out["id"].(string)

	var jobs int
	require.NoError(t, e.s.Store.Pool.QueryRow(context.Background(), "select count(*) from river_job where kind = 'generate_ad' and args->>'ad_id' in (select a.id::text from ads a where a.project_id::text = $1)", id).Scan(&jobs))
	require.Equal(t, 1, jobs, "job enqueued in the same transaction")

	code, p := e.do("GET", "/v1/projects/"+id, "")
	require.Equal(t, 200, code)
	require.Equal(t, "Merino Runner", p["product"].(map[string]any)["title"])
	require.Len(t, p["ads"], 1)

	code, out = e.do("POST", "/v1/projects", fmt.Sprintf(`{"url":%q}`, e.shop.URL+"/p/shoe"))
	require.Equal(t, 402, code)
	require.Equal(t, "no_credits", out["code"])

	other := setup(t)
	code, _ = other.do("GET", "/v1/projects/"+id, "")
	require.Equal(t, 404, code, "other users cannot read the project")
}

func TestModerationBlocksWithoutCharging(t *testing.T) {
	e := setup(t)
	code, out := e.do("POST", "/v1/projects", fmt.Sprintf(`{"url":%q}`, e.shop.URL+"/p/gun"))
	require.Equal(t, 422, code, out)
	_, me := e.do("GET", "/v1/me", "")
	require.EqualValues(t, 1, me["balance"])
}

func TestProductionScraperBlocksInternalURL(t *testing.T) {
	e := setup(t)
	e.s.Scraper = scrape.NewClient()
	code, _ := e.do("POST", "/v1/projects", fmt.Sprintf(`{"url":%q}`, e.shop.URL))
	require.Equal(t, 400, code)
	_, me := e.do("GET", "/v1/me", "")
	require.EqualValues(t, 1, me["balance"])
}

func TestBadInputs(t *testing.T) {
	e := setup(t)
	for _, body := range []string{``, `{`, `{"url":""}`, `{"url":"` + strings.Repeat("a", 3000) + `"}`, `{"url":"ftp://x"}`} {
		code, _ := e.do("POST", "/v1/projects", body)
		require.Equal(t, 400, code, body)
	}
	code, _ := e.do("POST", "/v1/billing/checkout", `{"plan":"free"}`)
	require.Contains(t, []int{400, 501}, code)
}

func TestAccountExportAndDelete(t *testing.T) {
	e := setup(t)
	e.do("GET", "/v1/me", "")
	code, out := e.do("GET", "/v1/account/export", "")
	require.Equal(t, 200, code)
	require.NotEmpty(t, out["ledger"])

	code, _ = e.do("DELETE", "/v1/account", `{"confirm":"wrong@x.com"}`)
	require.Equal(t, 400, code)
	_, me := e.do("GET", "/v1/me", "")
	code, _ = e.do("DELETE", "/v1/account", fmt.Sprintf(`{"confirm":%q}`, me["email"]))
	require.Equal(t, 200, code)
	var n int
	require.NoError(t, e.s.Store.Pool.QueryRow(context.Background(), "select count(*) from profiles where id::text = $1", me["id"]).Scan(&n))
	require.Zero(t, n)
}

func TestRateLimitOnProjects(t *testing.T) {
	e := setup(t)
	codes := map[int]int{}
	for range 33 {
		code, _ := e.do("POST", "/v1/projects", `{"url":""}`)
		codes[code]++
	}
	require.Positive(t, codes[429], codes)
}

func TestCORS(t *testing.T) {
	e := setup(t)
	_, _ = e.do("GET", "/v1/plans", "")
	req, _ := http.NewRequest("OPTIONS", e.srv.URL+"/v1/projects", nil)
	req.Header.Set("Origin", "http://app.test")
	resp, _ := http.DefaultClient.Do(req)
	require.Equal(t, 204, resp.StatusCode)
	require.Equal(t, "http://app.test", resp.Header.Get("Access-Control-Allow-Origin"))
	req.Header.Set("Origin", "http://evil.test")
	resp, _ = http.DefaultClient.Do(req)
	require.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
}

func sign(secret, id, ts, body string) string {
	key, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	m := hmac.New(sha256.New, key)
	fmt.Fprintf(m, "%s.%s.%s", id, ts, body)
	return "v1," + base64.StdEncoding.EncodeToString(m.Sum(nil))
}

func TestVerifyStandardWebhook(t *testing.T) {
	secret := "whsec_" + base64.StdEncoding.EncodeToString([]byte("hook-secret"))
	now := time.Now()
	ts := fmt.Sprint(now.Unix())
	body := `{"request_id":"r1"}`
	good := sign(secret, "evt1", ts, body)
	require.True(t, verifyStandardWebhook(secret, "evt1", ts, []byte(body), good, now))
	require.True(t, verifyStandardWebhook(secret, "evt1", ts, []byte(body), "v1,AAAA "+good, now), "any matching signature is accepted (rotation)")
	require.False(t, verifyStandardWebhook(secret, "evt1", ts, []byte(body+" "), good, now), "tampered body")
	require.False(t, verifyStandardWebhook(secret, "evt2", ts, []byte(body), good, now), "wrong id")
	require.False(t, verifyStandardWebhook(secret, "evt1", ts, []byte(body), good, now.Add(10*time.Minute)), "stale timestamp")
	require.False(t, verifyStandardWebhook("", "evt1", ts, []byte(body), good, now), "no secret configured")
}

func TestHiggsfieldWebhookEndpoint(t *testing.T) {
	e := setup(t)
	body := `{"request_id":"unknown"}`
	ts := fmt.Sprint(time.Now().Unix())
	code, _ := e.do("POST", "/v1/webhooks/higgsfield", body, "webhook-id", "e1", "webhook-timestamp", ts, "webhook-signature", sign(e.s.Cfg.HiggsfieldWebhookSecret, "e1", ts, body))
	require.Equal(t, 200, code)
	code, _ = e.do("POST", "/v1/webhooks/higgsfield", body, "webhook-id", "e1", "webhook-timestamp", ts, "webhook-signature", "v1,AAAA")
	require.Equal(t, 400, code)
}
