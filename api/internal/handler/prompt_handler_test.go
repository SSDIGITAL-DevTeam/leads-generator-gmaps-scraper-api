package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/service"
)

func TestPromptHandler_Validation(t *testing.T) {
	handler := &PromptSearchHandler{worker: &workerStub{}, service: service.NewPromptService("Indonesia")}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/prompt", strings.NewReader(`{"prompt":""}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.Enqueue(c); err != nil {
		t.Fatalf("expected handler to write response, got %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestPromptHandler_Success(t *testing.T) {
	worker := &workerStub{data: map[string]any{"status": "queued"}}
	handler := &PromptSearchHandler{worker: worker, service: service.NewPromptService("Indonesia")}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/prompt", strings.NewReader(`{"prompt":"cari PT di Jakarta"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.Enqueue(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
