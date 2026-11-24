package video

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
)

func provider(t *testing.T, h http.HandlerFunc) *Higgsfield {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Higgsfield{cfg: config.Config{HiggsfieldKeyID: "id", HiggsfieldKeySecret: "sec", HiggsfieldModel: "vendor/model", APIPublicURL: "https://app.example/api", HiggsfieldWebhookSecret: "whsec_x"}, http: srv.Client(), base: srv.URL}
}

var ad = store.Ad{ID: "ad-1", Hook: "hook", Script: "script", Avatar: "avatar"}

func TestStartSendsAuthIdempotencyAndWebhook(t *testing.T) {
	var gotAuth, gotIdem, gotQuery, gotPath string
	p := provider(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotIdem, gotQuery, gotPath = r.Header.Get("Authorization"), r.Header.Get("Idempotency-Key"), r.URL.RawQuery, r.URL.Path
		w.Write([]byte(`{"status":"queued","request_id":"req-9"}`))
	})
	id, err := p.Start(context.Background(), ad, store.Product{Title: "Shoe", Image: "https://img/x.jpg"})
	require.NoError(t, err)
	require.Equal(t, "req-9", id)
	require.Equal(t, "Key id:sec", gotAuth)
	require.Equal(t, "ad-1", gotIdem)
	require.Equal(t, "/vendor/model", gotPath)
	require.Contains(t, gotQuery, "hf_webhook=https%3A%2F%2Fapp.example%2Fapi%2Fv1%2Fwebhooks%2Fhiggsfield")
}

func TestStartErrorClasses(t *testing.T) {
	cases := []struct {
		status int
		header string
		check  func(error)
	}{
		{429, "17", func(err error) {
			var r *RetryLaterError
			require.True(t, errors.As(err, &r))
			require.Equal(t, 17*time.Second, r.After)
		}},
		{503, "", func(err error) {
			var r *RetryLaterError
			require.True(t, errors.As(err, &r))
			require.Equal(t, 30*time.Second, r.After)
		}},
		{422, "", func(err error) {
			var p *PermanentError
			require.True(t, errors.As(err, &p))
		}},
		{500, "", func(err error) {
			var p *PermanentError
			var r *RetryLaterError
			require.Error(t, err)
			require.False(t, errors.As(err, &p) || errors.As(err, &r))
		}},
	}
	for _, c := range cases {
		p := provider(t, func(w http.ResponseWriter, r *http.Request) {
			if c.header != "" {
				w.Header().Set("Retry-After", c.header)
			}
			w.WriteHeader(c.status)
			w.Write([]byte(`{"detail":"x"}`))
		})
		_, err := p.Start(context.Background(), ad, store.Product{})
		c.check(err)
	}
}

func TestPollStates(t *testing.T) {
	responses := map[string]struct {
		code int
		body string
		want Result
	}{
		"queued":    {200, `{"status":"queued"}`, Result{State: Processing}},
		"running":   {200, `{"status":"in_progress"}`, Result{State: Processing}},
		"done":      {200, `{"status":"completed","video":{"url":"https://cdn/v.mp4"}}`, Result{State: Done, VideoURL: "https://cdn/v.mp4"}},
		"no video":  {200, `{"status":"completed"}`, Result{State: Failed, Error: "No video returned"}},
		"nsfw":      {200, `{"status":"nsfw"}`, Result{State: Failed, Error: "Rejected by content moderation"}},
		"failed":    {200, `{"status":"failed","error":"boom"}`, Result{State: Failed, Error: "boom"}},
		"canceled":  {200, `{"status":"canceled"}`, Result{State: Failed, Error: "Generation canceled"}},
		"not found": {404, `{"detail":"nope"}`, Result{State: Failed, Error: "Generation request not found"}},
		"5xx":       {502, ``, Result{State: Processing}},
		"garbage":   {200, `<html>`, Result{State: Processing}},
	}
	for name, c := range responses {
		p := provider(t, func(w http.ResponseWriter, r *http.Request) {
			require.True(t, strings.HasSuffix(r.URL.Path, "/requests/req-1/status"))
			w.WriteHeader(c.code)
			w.Write([]byte(c.body))
		})
		got, err := p.Poll(context.Background(), "req-1")
		require.NoError(t, err, name)
		require.Equal(t, c.want, got, name)
	}
}

func TestMockCompletesAfterDelay(t *testing.T) {
	m := &Mock{}
	id, _ := m.Start(context.Background(), ad, store.Product{})
	r, _ := m.Poll(context.Background(), id)
	require.Equal(t, Processing, r.State)
	r, _ = m.Poll(context.Background(), "mock_1")
	require.Equal(t, Done, r.State)
}
