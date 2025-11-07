package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	if !repo.lastFilter.LatestRunOnly {
		t.Fatalf("expected latest run filter enabled")
	}
	if repo.lastFilter.Sort != "recent" {
		t.Fatalf("expected default sort recent, got %q", repo.lastFilter.Sort)
	}

	var payload APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Status != "success" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestCompaniesHandler_List_WithUpdatedSinceAndSort(t *testing.T) {
	repo := &capturingCompaniesRepo{}
	handler := newCompaniesHandler(repo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/companies?updated_since=2025-01-01T00:00:00Z&sort=recent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.List(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastFilter.UpdatedSince == nil {
		t.Fatalf("expected updated_since parsed")
	}
	if repo.lastFilter.Sort != "recent" {
		t.Fatalf("expected sort propagated, got %q", repo.lastFilter.Sort)
	}
	expected := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if !repo.lastFilter.UpdatedSince.Equal(expected) {
		t.Fatalf("expected updated_since %v, got %v", expected, repo.lastFilter.UpdatedSince)
	}
	if !repo.lastFilter.LatestRunOnly {
		t.Fatalf("expected latest run filter enabled")
	}
}

func TestCompaniesHandler_List_InvalidUpdatedSince(t *testing.T) {
	repo := &capturingCompaniesRepo{}
	handler := newCompaniesHandler(repo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/companies?updated_since=bad", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.List(c)
	if err != nil {
		t.Fatalf("handler should return response without error object, got %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
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

func TestCompaniesHandler_List_ScrapeRunIDFilter(t *testing.T) {
	repo := &capturingCompaniesRepo{}
	handler := newCompaniesHandler(repo)

	runID := "33333333-3333-4333-8333-333333333333"
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/companies?scrape_run_id="+runID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.List(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastFilter.ScrapeRunID == nil || repo.lastFilter.ScrapeRunID.String() != runID {
		t.Fatalf("expected scrape_run_id parsed")
	}
}

func TestCompaniesHandler_List_InvalidScrapeRunID(t *testing.T) {
	repo := &capturingCompaniesRepo{}
	handler := newCompaniesHandler(repo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/companies?scrape_run_id=not-a-uuid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.List(c); err != nil {
		t.Fatalf("expected handler to write response")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid scrape_run_id, got %d", rec.Code)
	}
}

func TestCompaniesHandler_ListAdmin_AllData(t *testing.T) {
	repo := &capturingCompaniesRepo{}
	handler := newCompaniesHandler(repo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/companies", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListAdmin(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastFilter.LatestRunOnly {
		t.Fatalf("expected admin listing to include all data")
	}
	if repo.lastFilter.Sort != "" {
		t.Fatalf("expected sort untouched for admin, got %q", repo.lastFilter.Sort)
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
