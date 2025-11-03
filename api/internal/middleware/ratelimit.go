package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/octobees/leads-generator/api/internal/config"
)

// ScrapeRateLimiter applies a token bucket limiter for the /scrape endpoint.
func ScrapeRateLimiter(cfg config.RateLimitConfig) echo.MiddlewareFunc {
	if cfg.Requests <= 0 || cfg.Interval <= 0 {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				return next(c)
			}
		}
	}

	perRequest := cfg.Interval / time.Duration(cfg.Requests)
	if perRequest <= 0 {
		perRequest = time.Second
	}

	limiter := rate.NewLimiter(rate.Every(perRequest), cfg.Requests)
	var mu sync.Mutex

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Path() != "/scrape" {
				return next(c)
			}

			mu.Lock()
			allowed := limiter.Allow()
			mu.Unlock()

			if !allowed {
				return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "scrape rate limit exceeded"})
			}

			return next(c)
		}
	}
}
