package middleware

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/config"
)

func TestLoggingMiddleware(t *testing.T) {
	orig := log.Writer()
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	defer log.SetOutput(orig)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(ContextKeyRequestID, "rid-123")

	err := Logging()(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(buf.String(), "request_id=rid-123") {
		t.Fatalf("expected log output to contain request id, got %s", buf.String())
	}

	// ensure errors are propagated and logged
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.Set(ContextKeyRequestID, "rid-456")
	expected := errors.New("boom")
	err = Logging()(func(c echo.Context) error {
		return expected
	})(c)
	if !strings.Contains(buf.String(), "rid-456") {
		t.Fatalf("expected second log entry with new request id")
	}
	if !errors.Is(err, expected) {
		t.Fatalf("expected error to bubble up")
	}
}

func TestScrapeRateLimiter(t *testing.T) {
	cfg := config.RateLimitConfig{Requests: 1, Interval: time.Second}
	mw := ScrapeRateLimiter(cfg)

	e := echo.New()
	nextCalls := 0
	next := func(c echo.Context) error {
		nextCalls++
		return c.NoContent(http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodPost, "/scrape", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/scrape")

	_ = mw(next)(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected first request to pass, got %d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/scrape", nil)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	c2.SetPath("/scrape")
	_ = mw(next)(c2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request rejected, got %d", rec2.Code)
	}

	// Non-scrape path should bypass limiter.
	req3 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec3 := httptest.NewRecorder()
	c3 := e.NewContext(req3, rec3)
	_ = mw(next)(c3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected non-scrape request to pass")
	}

	// zero config should behave as passthrough
	mw = ScrapeRateLimiter(config.RateLimitConfig{})
	req4 := httptest.NewRequest(http.MethodPost, "/scrape", nil)
	rec4 := httptest.NewRecorder()
	c4 := e.NewContext(req4, rec4)
	c4.SetPath("/scrape")
	_ = mw(next)(c4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected passthrough when limiter disabled")
	}
	if nextCalls == 0 {
		t.Fatalf("expected next handler to be invoked")
	}
}

func TestRequireRole(t *testing.T) {
	e := echo.New()
	mw := RequireRole("admin")

	t.Run("missing role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_ = mw(func(c echo.Context) error { return nil })(c)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("incorrect role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set(ContextKeyUserRole, "user")

		_ = mw(func(c echo.Context) error { return nil })(c)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set(ContextKeyUserRole, "admin")

		called := false
		if err := mw(func(c echo.Context) error {
			called = true
			return c.NoContent(http.StatusOK)
		})(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Fatalf("expected handler to run")
		}
	})
}

func TestRequestIDMiddleware(t *testing.T) {
	e := echo.New()
	handler := RequestID()

	t.Run("reuse incoming header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-ID", "incoming")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := handler(func(c echo.Context) error {
			if RequestIDFromContext(c) != "incoming" {
				t.Fatalf("expected request id to be stored")
			}
			return c.NoContent(http.StatusOK)
		})(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if rec.Header().Get("X-Request-ID") != "incoming" {
			t.Fatalf("expected response header to propagate request id")
		}
	})

	t.Run("generate when missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := handler(func(c echo.Context) error {
			rid := RequestIDFromContext(c)
			if rid == "" {
				t.Fatalf("expected generated request id")
			}
			return c.NoContent(http.StatusOK)
		})(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if rec.Header().Get("X-Request-ID") == "" {
			t.Fatalf("expected response header set")
		}
	})
}
