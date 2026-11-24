// Package config loads and validates process configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env    string
	Addr   string
	AppURL string
	// APIPublicURL is the externally reachable base of this API, used for provider webhook callbacks.
	APIPublicURL string
	LogLevel     string

	DatabaseURL string
	DatabaseSSL string
	DatabaseCA  string
	PoolMax     int32

	AllowedOrigins   []string
	TrustedProxyHops int

	SupabaseURL            string
	SupabaseJWTSecret      string
	SupabaseServiceRoleKey string
	DevAuth                bool
	DevJWTSecret           string

	StripeSecretKey     string
	StripeWebhookSecret string
	StripePrices        map[string]string

	S3Bucket          string
	S3Endpoint        string
	S3Region          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3ForcePathStyle  bool
	S3PublicURL       string
	S3Private         bool

	HiggsfieldKeyID         string
	HiggsfieldKeySecret     string
	HiggsfieldModel         string
	HiggsfieldWebhookSecret string

	OpenAIKey   string
	OpenAIModel string

	MaxProjectsPerHour int
	MaxAdsPerDay       int
	WorkerConcurrency  int
	ErrorWebhookURL    string
	MetricsToken       string
	AdMaxAge           time.Duration
}

func (c Config) Production() bool { return c.Env == "production" }
func (c Config) StorageEnabled() bool {
	return c.S3Bucket != "" && c.S3AccessKeyID != "" && c.S3SecretAccessKey != ""
}
func (c Config) HiggsfieldEnabled() bool {
	return c.HiggsfieldKeyID != "" && c.HiggsfieldKeySecret != "" && c.HiggsfieldModel != ""
}
func (c Config) BillingEnabled() bool { return c.StripeSecretKey != "" }

func Load() Config {
	env := get("APP_ENV", get("NODE_ENV", "development"))
	c := Config{
		Env:      env,
		Addr:     get("ADDR", ":"+get("PORT", "8080")),
		AppURL:   strings.TrimRight(os.Getenv("APP_URL"), "/"),
		LogLevel: get("LOG_LEVEL", "info"),

		DatabaseURL: os.Getenv("DATABASE_URL"),
		DatabaseSSL: get("DATABASE_SSL", map[bool]string{true: "require", false: "off"}[env == "production"]),
		DatabaseCA:  os.Getenv("DATABASE_CA"),
		PoolMax:     int32(geti("PG_POOL_MAX", 10)),

		AllowedOrigins:   split(os.Getenv("ALLOWED_ORIGINS")),
		TrustedProxyHops: geti("TRUSTED_PROXY_HOPS", 1),

		SupabaseURL:            strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		SupabaseJWTSecret:      os.Getenv("SUPABASE_JWT_SECRET"),
		SupabaseServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		DevJWTSecret:           get("DEV_JWT_SECRET", "dev-only-secret-change-me-32-bytes-min"),

		StripeSecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripePrices: map[string]string{
			"starter": os.Getenv("STRIPE_PRICE_STARTER"),
			"growth":  os.Getenv("STRIPE_PRICE_GROWTH"),
			"scale":   os.Getenv("STRIPE_PRICE_SCALE"),
		},

		S3Bucket:          os.Getenv("S3_BUCKET"),
		S3Endpoint:        os.Getenv("S3_ENDPOINT"),
		S3Region:          get("S3_REGION", "auto"),
		S3AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
		S3SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		S3ForcePathStyle:  os.Getenv("S3_FORCE_PATH_STYLE") != "",
		S3PublicURL:       strings.TrimRight(os.Getenv("S3_PUBLIC_URL"), "/"),
		S3Private:         os.Getenv("S3_PRIVATE") != "0",

		HiggsfieldKeyID:         os.Getenv("HIGGSFIELD_KEY_ID"),
		HiggsfieldKeySecret:     os.Getenv("HIGGSFIELD_KEY_SECRET"),
		HiggsfieldModel:         strings.Trim(os.Getenv("HIGGSFIELD_MODEL"), "/"),
		HiggsfieldWebhookSecret: os.Getenv("HIGGSFIELD_WEBHOOK_SECRET"),

		OpenAIKey:   os.Getenv("OPENAI_API_KEY"),
		OpenAIModel: get("OPENAI_MODEL", "gpt-4o-mini"),

		MaxProjectsPerHour: geti("MAX_PROJECTS_PER_HOUR", 10),
		MaxAdsPerDay:       geti("MAX_ADS_PER_DAY", 30),
		WorkerConcurrency:  geti("WORKER_CONCURRENCY", 5),
		ErrorWebhookURL:    os.Getenv("ERROR_WEBHOOK_URL"),
		MetricsToken:       os.Getenv("METRICS_TOKEN"),
		AdMaxAge:           25 * time.Minute,
	}
	c.APIPublicURL = strings.TrimRight(get("API_PUBLIC_URL", c.AppURL+"/api"), "/")
	c.DevAuth = c.Env != "production" && c.SupabaseURL == ""
	return c
}

// Problems returns every configuration problem so operators can fix them in one pass.
func (c Config) Problems() []string {
	var p []string
	if c.DatabaseURL == "" {
		p = append(p, "DATABASE_URL is required")
	}
	if c.Production() {
		if c.SupabaseURL == "" {
			p = append(p, "SUPABASE_URL is required in production")
		}
		if c.SupabaseJWTSecret == "" && c.SupabaseURL == "" {
			p = append(p, "SUPABASE_JWT_SECRET or SUPABASE_URL (JWKS) is required in production")
		}
		if !strings.HasPrefix(c.AppURL, "https://") {
			p = append(p, "APP_URL must be an https URL in production")
		}
		if !c.BillingEnabled() {
			p = append(p, "Stripe is not configured")
		}
		if !c.StorageEnabled() {
			p = append(p, "S3 storage is not configured")
		}
	}
	if c.BillingEnabled() {
		for k, v := range map[string]string{"STRIPE_WEBHOOK_SECRET": c.StripeWebhookSecret, "STRIPE_PRICE_STARTER": c.StripePrices["starter"], "STRIPE_PRICE_GROWTH": c.StripePrices["growth"], "STRIPE_PRICE_SCALE": c.StripePrices["scale"]} {
			if v == "" {
				p = append(p, k+" is required when STRIPE_SECRET_KEY is set")
			}
		}
	}
	if (c.S3Bucket != "" || c.S3AccessKeyID != "" || c.S3SecretAccessKey != "") && !c.StorageEnabled() {
		p = append(p, "S3 storage is partly configured (need S3_BUCKET, S3_ACCESS_KEY_ID, S3_SECRET_ACCESS_KEY)")
	}
	if c.StorageEnabled() && !c.S3Private && c.S3PublicURL == "" {
		p = append(p, "S3_PUBLIC_URL is required when S3_PRIVATE=0")
	}
	h := []string{c.HiggsfieldKeyID, c.HiggsfieldKeySecret, c.HiggsfieldModel}
	n := 0
	for _, v := range h {
		if v != "" {
			n++
		}
	}
	if n > 0 && n < 3 {
		p = append(p, "Higgsfield is partly configured (need HIGGSFIELD_KEY_ID, HIGGSFIELD_KEY_SECRET, HIGGSFIELD_MODEL)")
	}
	return p
}

func (c Config) Validate() error {
	if p := c.Problems(); len(p) > 0 {
		return fmt.Errorf("invalid configuration:\n- %s", strings.Join(p, "\n- "))
	}
	return nil
}

func get(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func geti(k string, d int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return d
}

func split(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
