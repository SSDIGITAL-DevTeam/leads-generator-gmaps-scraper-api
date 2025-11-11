package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func newEnrichHandlerWithWorker(worker WorkerPoster) *EnrichWorkerHandler {
	return &EnrichWorkerHandler{worker: worker}
}

func TestEnrichWorkerHandler_Validation(t *testing.T) {
	e := echo.New()
	handler := newEnrichHandlerWithWorker(&workerStub{data: map[string]any{"status": "queued"}})

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

		handler := newEnrichHandlerWithWorker(&workerStub{err: fmt.Errorf("network")})

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

		handler := newEnrichHandlerWithWorker(&workerStub{err: fmt.Errorf("boom")})

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/enrich", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newEnrichHandlerWithWorker(&workerStub{data: map[string]any{"status": "queued"}})

		_ = handler.Enqueue(c)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})
}
