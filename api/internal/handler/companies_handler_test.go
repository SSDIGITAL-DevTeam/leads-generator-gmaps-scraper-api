package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/entity"
	"github.com/octobees/leads-generator/api/internal/repository"
	"github.com/octobees/leads-generator/api/internal/service"
)

type capturingCompaniesRepo struct {
	lastFilter dto.ListFilter
	err        error
}

func (c *capturingCompaniesRepo) List(ctx context.Context, filter dto.ListFilter) ([]entity.Company, error) {
	c.lastFilter = filter
	if c.err != nil {
		return nil, c.err
	}
	return []entity.Company{{Company: "Acme"}}, nil
}

func (c *capturingCompaniesRepo) BulkUpsertCompanies(ctx context.Context, records []repository.BulkUpsertCompanyInput) (repository.BulkUpsertResult, error) {
	return repository.BulkUpsertResult{}, nil
}

func (c *capturingCompaniesRepo) Upsert(ctx context.Context, company *entity.Company) error {
	return nil
}

func newCompaniesHandler(repo repository.CompaniesRepository) *CompaniesHandler {
	return NewCompaniesHandler(service.NewCompaniesService(repo))
}

func TestCompaniesHandler_List_Success(t *testing.T) {
	repo := &capturingCompaniesRepo{}
	handler := newCompaniesHandler(repo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/companies?q=plumber&per_page=25&min_rating=4.5", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.List(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if repo.lastFilter.Q != "plumber" {
		t.Fatalf("expected query filter applied")
	}
	if repo.lastFilter.PerPage != 25 {
		t.Fatalf("expected per_page 25, got %d", repo.lastFilter.PerPage)
	}
	if repo.lastFilter.MinRating == nil || *repo.lastFilter.MinRating != 4.5 {
		t.Fatalf("expected min_rating parsed, got %v", repo.lastFilter.MinRating)
	}

	var payload APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Status != "success" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestCompaniesHandler_List_Error(t *testing.T) {
	repo := &capturingCompaniesRepo{err: context.DeadlineExceeded}
	handler := newCompaniesHandler(repo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/companies", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = handler.List(c)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestCompaniesHandler_parseIntDefault(t *testing.T) {
	if val := parseIntDefault("", 5); val != 5 {
		t.Fatalf("expected fallback when empty")
	}
	if val := parseIntDefault("10", 5); val != 10 {
		t.Fatalf("expected parsed value, got %d", val)
	}
	if val := parseIntDefault("bad", 5); val != 5 {
		t.Fatalf("expected fallback on parse error")
	}
}
