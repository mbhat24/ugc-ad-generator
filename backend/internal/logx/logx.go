// Package logx configures structured logging and optional error alerting.
package logx

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type alertHandler struct {
	slog.Handler
	url  string
	last atomic.Int64
}

func (h *alertHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError && h.url != "" {
		now := time.Now().UnixMilli()
		if prev := h.last.Load(); now-prev > 30000 && h.last.CompareAndSwap(prev, now) {
			text := "AdReel error: " + r.Message
			r.Attrs(func(a slog.Attr) bool {
				if a.Key == "err" {
					text += " " + a.Value.String()
				}
				return true
			})
			go func() {
				b, _ := json.Marshal(map[string]string{"text": truncate(text, 500)})
				c := &http.Client{Timeout: 5 * time.Second}
				if resp, err := c.Post(h.url, "application/json", bytes.NewReader(b)); err == nil {
					resp.Body.Close()
				}
			}()
		}
	}
	return h.Handler.Handle(ctx, r)
}

func (h *alertHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return &alertHandler{Handler: h.Handler.WithAttrs(a), url: h.url}
}
func (h *alertHandler) WithGroup(n string) slog.Handler {
	return &alertHandler{Handler: h.Handler.WithGroup(n), url: h.url}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// Setup installs a JSON logger as the process default.
func Setup(level, service, alertURL string) *slog.Logger {
	var lv slog.Level
	_ = lv.UnmarshalText([]byte(strings.ToUpper(level)))
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lv})
	l := slog.New(&alertHandler{Handler: base, url: alertURL}).With("service", service)
	slog.SetDefault(l)
	return l
}
