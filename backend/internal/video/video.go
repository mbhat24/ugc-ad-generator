// Package video abstracts AI video providers behind a small interface.
package video

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
)

type State int

const (
	Processing State = iota
	Done
	Failed
)

type Result struct {
	State    State
	VideoURL string
	Error    string
}

// RetryLaterError means the provider is busy; the job should be retried after the delay without failing the ad.
type RetryLaterError struct{ After time.Duration }

func (e *RetryLaterError) Error() string { return "provider busy, retry in " + e.After.String() }

// PermanentError marks failures that will not succeed on retry (for example a rejected request).
type PermanentError struct{ Msg string }

func (e *PermanentError) Error() string { return e.Msg }

type Provider interface {
	Start(ctx context.Context, ad store.Ad, p store.Product) (jobID string, err error)
	Poll(ctx context.Context, jobID string) (Result, error)
	Cleanup(ctx context.Context, jobID string)
	Name() string
}

func New(cfg config.Config) Provider {
	if cfg.HiggsfieldEnabled() {
		return &Higgsfield{cfg: cfg, http: &http.Client{Timeout: 30 * time.Second}, base: "https://api.higgsfield.ai"}
	}
	return &Mock{}
}

// ---- mock ----

type Mock struct{}

const mockVideo = "https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4"

func (*Mock) Name() string { return "mock" }
func (*Mock) Start(context.Context, store.Ad, store.Product) (string, error) {
	return fmt.Sprintf("mock_%d", time.Now().UnixMilli()), nil
}
func (*Mock) Poll(_ context.Context, id string) (Result, error) {
	ms, _ := strconv.ParseInt(strings.TrimPrefix(id, "mock_"), 10, 64)
	if time.Now().UnixMilli()-ms > 6000 {
		return Result{State: Done, VideoURL: mockVideo}, nil
	}
	return Result{State: Processing}, nil
}
func (*Mock) Cleanup(context.Context, string) {}

// ---- higgsfield ----

type Higgsfield struct {
	cfg  config.Config
	http *http.Client
	base string
}

func (h *Higgsfield) Name() string { return "higgsfield" }

func (h *Higgsfield) do(ctx context.Context, method, url string, body []byte, hdr map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Key "+h.cfg.HiggsfieldKeyID+":"+h.cfg.HiggsfieldKeySecret)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return h.http.Do(req)
}

func (h *Higgsfield) webhookURL() string {
	if strings.HasPrefix(h.cfg.APIPublicURL, "https://") && h.cfg.HiggsfieldWebhookSecret != "" {
		return h.cfg.APIPublicURL + "/v1/webhooks/higgsfield"
	}
	return ""
}

func (h *Higgsfield) Start(ctx context.Context, ad store.Ad, p store.Product) (string, error) {
	prompt := fmt.Sprintf(`Vertical 9:16 UGC-style selfie video. %s. Holding and showing "%s" to the camera, speaking naturally: "%s. %s"`, ad.Avatar, p.Title, ad.Hook, ad.Script)
	body := map[string]any{"prompt": prompt, "aspect_ratio": "9:16", "duration": 15}
	if p.Image != "" {
		body["image_url"] = p.Image
	}
	b, _ := json.Marshal(body)
	endpoint := h.base + "/" + h.cfg.HiggsfieldModel
	if w := h.webhookURL(); w != "" {
		endpoint += "?hf_webhook=" + url.QueryEscape(w)
	}
	resp, err := h.do(ctx, http.MethodPost, endpoint, b, map[string]string{"Idempotency-Key": ad.ID})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		after, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
		if after <= 0 {
			after = 30
		}
		return "", &RetryLaterError{After: time.Duration(after) * time.Second}
	}
	if resp.StatusCode/100 == 4 {
		return "", &PermanentError{Msg: fmt.Sprintf("Higgsfield rejected the request (%d): %s", resp.StatusCode, truncate(string(raw), 200))}
	}
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("higgsfield error %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var out struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.RequestID == "" {
		return "", errors.New("higgsfield returned no request id")
	}
	return out.RequestID, nil
}

func (h *Higgsfield) Poll(ctx context.Context, id string) (Result, error) {
	resp, err := h.do(ctx, http.MethodGet, h.base+"/requests/"+url.PathEscape(id)+"/status", nil, nil)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Result{State: Failed, Error: "Generation request not found"}, nil
	}
	if resp.StatusCode/100 != 2 {
		return Result{State: Processing}, nil
	}
	var out struct {
		Status string  `json:"status"`
		Error  *string `json:"error"`
		Video  *struct {
			URL string `json:"url"`
		} `json:"video"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return Result{State: Processing}, nil
	}
	switch out.Status {
	case "completed":
		if out.Video == nil || out.Video.URL == "" {
			return Result{State: Failed, Error: "No video returned"}, nil
		}
		return Result{State: Done, VideoURL: out.Video.URL}, nil
	case "nsfw":
		return Result{State: Failed, Error: "Rejected by content moderation"}, nil
	case "failed", "canceled":
		msg := "Generation " + out.Status
		if out.Error != nil && *out.Error != "" {
			msg = *out.Error
		}
		return Result{State: Failed, Error: msg}, nil
	}
	return Result{State: Processing}, nil
}

func (h *Higgsfield) Cleanup(ctx context.Context, id string) {
	if resp, err := h.do(ctx, http.MethodDelete, h.base+"/requests/"+url.PathEscape(id), nil, nil); err == nil {
		resp.Body.Close()
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
