package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// RateLimitConfig indicates how many requests are allowed within a given interval.
type RateLimitConfig struct {
	Requests int
	Interval time.Duration
}

// Config aggregates application-wide configuration values.
type Config struct {
	DatabaseURL     string
	JWTSecret       string
	Port            string
	WorkerBaseURL   string
	RateLimitScrape RateLimitConfig
	TokenTTL        time.Duration
}

// Load reads configuration from environment variables and applies sane defaults.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     getEnv("JWT_SECRET", "dev-secret"),
		Port:          getEnv("PORT", "8080"),
		WorkerBaseURL: getEnv("WORKER_BASE_URL", "http://worker:9000"),
		TokenTTL:      parseDuration(getEnv("JWT_TTL", "24h")),
	}

	rl, err := parseRateLimit(getEnv("RATE_LIMIT_SCRAPE", "5/min"))
	if err != nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT_SCRAPE value: %w", err)
	}
	cfg.RateLimitScrape = rl

	return cfg, nil
}

func parseRateLimit(value string) (RateLimitConfig, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return RateLimitConfig{}, fmt.Errorf("expected format <requests>/<interval>, got %q", value)
	}

	requests, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || requests <= 0 {
		return RateLimitConfig{}, fmt.Errorf("invalid request count: %v", parts[0])
	}

	unit := strings.ToLower(strings.TrimSpace(parts[1]))
	var interval time.Duration
	switch unit {
	case "s", "sec", "second", "seconds":
		interval = time.Second
	case "m", "min", "minute", "minutes":
		interval = time.Minute
	case "h", "hr", "hour", "hours":
		interval = time.Hour
	default:
		return RateLimitConfig{}, fmt.Errorf("unsupported interval unit: %s", unit)
	}

	return RateLimitConfig{Requests: requests, Interval: interval}, nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func parseDuration(input string) time.Duration {
	d, err := time.ParseDuration(input)
	if err != nil {
		return 24 * time.Hour
	}
	return d
}
