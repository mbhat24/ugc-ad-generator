// Package httpx holds HTTP helpers and middleware shared by all handlers.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
)

type ctxKey int

const requestIDKey ctxKey = 1

type ErrorBody struct {
	Error string         `json:"error"`
	Code  string         `json:"code,omitempty"`
	Extra map[string]any `json:"extra,omitempty"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, msg, code string) {
	JSON(w, status, ErrorBody{Error: msg, Code: code})
}

var ErrBodyTooLarge = errors.New("request body too large")

// DecodeJSON reads a size-limited JSON object and rejects unknown trailing data.
func DecodeJSON(r *http.Request, dst any, maxBytes int64) error {
	b, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if int64(len(b)) > maxBytes {
		return ErrBodyTooLarge
	}
	return json.Unmarshal(b, dst)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// ClientIP picks the client address assuming `hops` trusted reverse proxies append to X-Forwarded-For.
func ClientIP(r *http.Request, hops int) string {
	if hops > 0 {
		parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		var clean []string
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				clean = append(clean, p)
			}
		}
		if len(clean) >= hops {
			return clean[len(clean)-hops]
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }

var (
	reqTotal = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "adreel_http_requests_total", Help: "HTTP requests"}, []string{"method", "route", "status"})
	reqDur   = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "adreel_http_request_duration_seconds", Help: "HTTP latency", Buckets: prometheus.DefBuckets}, []string{"method", "route"})
)

func init() { prometheus.MustRegister(reqTotal, reqDur) }

// Observe assigns a request id, recovers from panics, records metrics and writes an access log.
func Observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" || len(id) > 64 {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey, id))
		sw := &statusWriter{ResponseWriter: w, status: 200}
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic", "request_id", id, "err", rec, "stack", string(debug.Stack()))
				if sw.status == 200 {
					Error(sw, http.StatusInternalServerError, "Internal error", "internal")
				}
			}
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			d := time.Since(start)
			reqTotal.WithLabelValues(r.Method, route, strconv.Itoa(sw.status)).Inc()
			reqDur.WithLabelValues(r.Method, route).Observe(d.Seconds())
			if route != "/healthz" && route != "/readyz" && route != "/metrics" {
				lvl := slog.LevelInfo
				if sw.status >= 500 {
					lvl = slog.LevelError
				}
				slog.Log(r.Context(), lvl, "request", "request_id", id, "method", r.Method, "route", route, "status", sw.status, "ms", d.Milliseconds())
			}
		}()
		next.ServeHTTP(sw, r)
	})
}

func SecurityHeaders(prod bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Cache-Control", "no-store")
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			if prod {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func CORS(origins []string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if o := r.Header.Get("Origin"); o != "" && allowed[o] {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", o)
				h.Set("Vary", "Origin")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
				h.Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				h.Set("Access-Control-Max-Age", "600")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
