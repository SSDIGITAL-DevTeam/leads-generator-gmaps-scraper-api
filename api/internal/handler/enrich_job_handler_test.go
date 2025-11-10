package handler

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	middlewarepkg "github.com/octobees/leads-generator/api/internal/middleware"
)

func newTestEnrichJobHandler(t *testing.T, rt roundTripFunc, baseURL string) *EnrichWorkerHandler {
	t.Helper()
	client := &http.Client{Transport: rt}
	return NewEnrichWorkerHandler(client, baseURL)
}

func TestEnrichWorkerHandler_Validation(t *testing.T) {
	e := echo.New()
	handler := newTestEnrichJobHandler(t, func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":{"status":"queued"}}`))}, nil
	}, "http://worker")

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/enrich", strings.NewReader("{"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/enrich", strings.NewReader(`{"company_id":""}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("panic when base url missing", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic")
			}
		}()

		_ = NewEnrichWorkerHandler(&http.Client{}, "")
	})
}

func TestEnrichWorkerHandler_WorkerResponses(t *testing.T) {
	e := echo.New()
	body := `{"company_id":"abc","website":"https://example.com"}`

	t.Run("network error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/enrich", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newTestEnrichJobHandler(t, func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("network")
		}, "http://worker")

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
	})

	t.Run("worker returns error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/enrich", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set(middlewarepkg.ContextKeyRequestID, "req-1")

		var captured string
		handler := newTestEnrichJobHandler(t, func(req *http.Request) (*http.Response, error) {
			captured = req.Header.Get("X-Request-ID")
			return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(`{"error":"boom"}`))}, nil
		}, "http://worker")

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
		if captured != "req-1" {
			t.Fatalf("expected request id propagation")
		}
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/enrich", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newTestEnrichJobHandler(t, func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":{"status":"queued"}}`))}, nil
		}, "http://worker")

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})
}
