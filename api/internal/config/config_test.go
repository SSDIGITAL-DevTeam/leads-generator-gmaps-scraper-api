package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	t.Setenv("JWT_SECRET", "super-secret")
	t.Setenv("PORT", "9000")
	t.Setenv("WORKER_BASE_URL", "http://worker")
	t.Setenv("JWT_TTL", "2h")
	t.Setenv("RATE_LIMIT_SCRAPE", "10/min")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost/db" {
		t.Fatalf("unexpected database url: %s", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "super-secret" || cfg.Port != "9000" || cfg.WorkerBaseURL != "http://worker" {
		t.Fatalf("unexpected config values: %+v", cfg)
	}
	if cfg.TokenTTL != 2*time.Hour {
		t.Fatalf("expected token ttl 2h, got %s", cfg.TokenTTL)
	}
	if cfg.RateLimitScrape.Requests != 10 || cfg.RateLimitScrape.Interval != time.Minute {
		t.Fatalf("unexpected rate limit config: %+v", cfg.RateLimitScrape)
	}

	// invalid rate limit should error
	os.Unsetenv("RATE_LIMIT_SCRAPE")
	t.Setenv("RATE_LIMIT_SCRAPE", "xyz")
	if _, err := Load(); err == nil {
		t.Fatalf("expected error for invalid rate limit")
	}
}

func TestParseRateLimit(t *testing.T) {
	cfg, err := parseRateLimit("5/sec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Requests != 5 || cfg.Interval != time.Second {
		t.Fatalf("unexpected config: %+v", cfg)
	}

	if _, err := parseRateLimit("bad-format"); err == nil {
		t.Fatalf("expected error for malformed value")
	}
	if _, err := parseRateLimit("0/min"); err == nil {
		t.Fatalf("expected error for zero requests")
	}
	if _, err := parseRateLimit("5/day"); err == nil {
		t.Fatalf("expected error for unsupported unit")
	}
}

func TestGetEnv(t *testing.T) {
	os.Unsetenv("FOO")
	if val := getEnv("FOO", "fallback"); val != "fallback" {
		t.Fatalf("expected fallback, got %s", val)
	}
	t.Setenv("FOO", "value")
	if val := getEnv("FOO", "fallback"); val != "value" {
		t.Fatalf("expected env value, got %s", val)
	}
}

func TestParseDuration(t *testing.T) {
	if parseDuration("3h") != 3*time.Hour {
		t.Fatalf("expected 3h duration")
	}
	if parseDuration("invalid") != 24*time.Hour {
		t.Fatalf("expected fallback duration")
	}
}
