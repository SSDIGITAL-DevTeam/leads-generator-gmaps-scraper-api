package handler

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	middlewarepkg "github.com/octobees/leads-generator/api/internal/middleware"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestScrapeHandler(rt roundTripFunc, baseURL string) *ScrapeHandler {
	client := &http.Client{Transport: rt}
	return NewScrapeHandler(client, baseURL)
}

func TestScrapeHandler_ValidationErrors(t *testing.T) {
	e := echo.New()
	handler := newTestScrapeHandler(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":{"status":"queued"}}`))}, nil
	}, "http://worker")

	t.Run("invalid payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/scrape", bytes.NewBufferString("{"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing type business", func(t *testing.T) {
		body := `{"city":"Gotham","country":"USA"}`
		req := httptest.NewRequest(http.MethodPost, "/scrape", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing city and country after fallback", func(t *testing.T) {
		body := `{"type_business":"plumber","location":"only-one-part"}`
		req := httptest.NewRequest(http.MethodPost, "/scrape", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 when city/country missing, got %d", rec.Code)
		}
	})

	t.Run("worker base url missing", func(t *testing.T) {
		body := `{"type_business":"plumber","city":"Gotham","country":"USA"}`
		req := httptest.NewRequest(http.MethodPost, "/scrape", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newTestScrapeHandler(func(req *http.Request) (*http.Response, error) {
			return nil, nil
		}, "")
		_ = handler.Enqueue(c)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 when base url missing, got %d", rec.Code)
		}
	})
}

func TestScrapeHandler_WorkerInteraction(t *testing.T) {
	e := echo.New()

	t.Run("worker request failure", func(t *testing.T) {
		body := `{"type_business":"plumber","city":"Gotham","country":"USA"}`
		req := httptest.NewRequest(http.MethodPost, "/scrape", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newTestScrapeHandler(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		}, "http://worker")

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
	})

	t.Run("worker returns error payload", func(t *testing.T) {
		body := `{"type_business":"plumber","city":"Gotham","country":"USA"}`
		req := httptest.NewRequest(http.MethodPost, "/scrape", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set(middlewarepkg.ContextKeyRequestID, "req-123")

		var capturedHeader string
		handler := newTestScrapeHandler(func(req *http.Request) (*http.Response, error) {
			capturedHeader = req.Header.Get("X-Request-ID")
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(strings.NewReader(`{"error":"worker failed"}`)),
			}, nil
		}, "http://worker")

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
		if capturedHeader != "req-123" {
			t.Fatalf("expected request id propagated, got %q", capturedHeader)
		}
	})

	t.Run("worker success without data", func(t *testing.T) {
		body := `{"type_business":"plumber","city":"Gotham","country":"USA"}`
		req := httptest.NewRequest(http.MethodPost, "/scrape", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newTestScrapeHandler(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}, "http://worker")

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("worker success with data", func(t *testing.T) {
		body := `{"type_business":"plumber","city":"Gotham","country":"USA","min_rating":4.5}`
		req := httptest.NewRequest(http.MethodPost, "/scrape", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newTestScrapeHandler(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"data":{"status":"queued"}}`)),
			}, nil
		}, "http://worker")

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})
}

func TestExtractWorkerError(t *testing.T) {
	msg := extractWorkerError(strings.NewReader(`{"error":"boom"}`))
	if msg != "boom" {
		t.Fatalf("expected boom, got %s", msg)
	}

	msg = extractWorkerError(strings.NewReader(`not-json`))
	if msg != "not-json" {
		t.Fatalf("expected raw body fallback, got %s", msg)
	}

	msg = extractWorkerError(bytes.NewReader(nil))
	if msg != "worker returned an error" {
		t.Fatalf("expected default message, got %s", msg)
	}
}
