package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/entity"
	"github.com/octobees/leads-generator/api/internal/repository"
	"github.com/octobees/leads-generator/api/internal/service"
)

type enrichmentRepoStub struct {
	saved *entity.CompanyEnrichment
	err   error
}

func (s *enrichmentRepoStub) List(ctx context.Context, filter dto.ListFilter) ([]entity.Company, error) {
	return nil, nil
}

func (s *enrichmentRepoStub) BulkUpsertCompanies(ctx context.Context, records []repository.BulkUpsertCompanyInput) (repository.BulkUpsertResult, error) {
	return repository.BulkUpsertResult{}, nil
}

func (s *enrichmentRepoStub) Upsert(ctx context.Context, company *entity.Company) error {
	return nil
}

func (s *enrichmentRepoStub) UpsertEnrichment(ctx context.Context, enrichment *entity.CompanyEnrichment) error {
	if s.err != nil {
		return s.err
	}
	s.saved = enrichment
	return nil
}

func TestEnrichHandler_SaveResult_Success(t *testing.T) {
	repo := &enrichmentRepoStub{}
	handler := NewEnrichHandler(service.NewCompaniesService(repo))

	e := echo.New()
	body := `{"company_id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","emails":["info@example.com"],"website":"https://acme.com"}`
	req := httptest.NewRequest(http.MethodPost, "/enrich-result", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.SaveResult(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if repo.saved == nil || repo.saved.CompanyID.String() != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
		t.Fatalf("expected enrichment saved, got %+v", repo.saved)
	}
}

func TestEnrichHandler_SaveResult_InvalidJSON(t *testing.T) {
	repo := &enrichmentRepoStub{}
	handler := NewEnrichHandler(service.NewCompaniesService(repo))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/enrich-result", strings.NewReader("not-json"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.SaveResult(c); err != nil {
		t.Fatalf("handler should handle error response without returning err: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestEnrichHandler_SaveResult_MissingCompanyID(t *testing.T) {
	repo := &enrichmentRepoStub{}
	handler := NewEnrichHandler(service.NewCompaniesService(repo))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/enrich-result", strings.NewReader(`{"emails":[]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.SaveResult(c); err != nil {
		t.Fatalf("handler should write response: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestEnrichHandler_SaveResult_ServiceErrors(t *testing.T) {
	repo := &enrichmentRepoStub{}
	handler := NewEnrichHandler(service.NewCompaniesService(repo))

	// Invalid UUID -> ErrInvalidCompanyID -> 400
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/enrich-result", strings.NewReader(`{"company_id":"bad"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.SaveResult(c); err != nil {
		t.Fatalf("handler should write response: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid uuid, got %d", rec.Code)
	}

	// Repository failure -> 500
	repo.err = errors.New("boom")
	req = httptest.NewRequest(http.MethodPost, "/enrich-result", strings.NewReader(`{"company_id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	if err := handler.SaveResult(c); err != nil {
		t.Fatalf("handler should write response: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when service fails, got %d", rec.Code)
	}
}
