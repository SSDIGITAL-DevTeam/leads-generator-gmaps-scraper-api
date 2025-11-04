package handler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/entity"
	"github.com/octobees/leads-generator/api/internal/repository"
	"github.com/octobees/leads-generator/api/internal/service"
)

type stubCompaniesRepository struct {
	bulk func(ctx context.Context, records []repository.BulkUpsertCompanyInput) (repository.BulkUpsertResult, error)
}

func (s *stubCompaniesRepository) List(ctx context.Context, filter dto.ListFilter) ([]entity.Company, error) {
	return nil, nil
}

func (s *stubCompaniesRepository) BulkUpsertCompanies(ctx context.Context, records []repository.BulkUpsertCompanyInput) (repository.BulkUpsertResult, error) {
	if s.bulk != nil {
		return s.bulk(ctx, records)
	}
	return repository.BulkUpsertResult{}, nil
}

func (s *stubCompaniesRepository) Upsert(ctx context.Context, company *entity.Company) error {
	return nil
}

func newAdminUploadHandler(repo repository.CompaniesRepository) *AdminUploadHandler {
	service := service.NewCompaniesService(repo)
	return NewAdminUploadHandler(service)
}

func TestAdminUploadHandler_MissingFile(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/admin/upload-csv", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := newAdminUploadHandler(&stubCompaniesRepository{})
	_ = handler.UploadCSV(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAdminUploadHandler_InvalidCSV(t *testing.T) {
	e := echo.New()
	req, rec := multipartRequest(t, "file", "test.csv", "company,address\nAcme,Main St\n")
	c := e.NewContext(req, rec)

	handler := newAdminUploadHandler(&stubCompaniesRepository{})
	_ = handler.UploadCSV(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid csv, got %d", rec.Code)
	}
}

func TestAdminUploadHandler_RepositoryError(t *testing.T) {
	e := echo.New()
	req, rec := multipartRequest(t, "file", "test.csv", validCSV())
	c := e.NewContext(req, rec)

	handler := newAdminUploadHandler(&stubCompaniesRepository{
		bulk: func(ctx context.Context, records []repository.BulkUpsertCompanyInput) (repository.BulkUpsertResult, error) {
			return repository.BulkUpsertResult{}, context.DeadlineExceeded
		},
	})

	_ = handler.UploadCSV(c)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestAdminUploadHandler_Success(t *testing.T) {
	e := echo.New()
	req, rec := multipartRequest(t, "file", "test.csv", validCSV())
	c := e.NewContext(req, rec)

	handler := newAdminUploadHandler(&stubCompaniesRepository{
		bulk: func(ctx context.Context, records []repository.BulkUpsertCompanyInput) (repository.BulkUpsertResult, error) {
			if len(records) != 1 {
				t.Fatalf("expected 1 record, got %d", len(records))
			}
			return repository.BulkUpsertResult{Inserted: 1, Total: 1}, nil
		},
	})

	_ = handler.UploadCSV(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func multipartRequest(t *testing.T, field, filename, content string) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/admin/upload-csv", body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	return req, rec
}

func validCSV() string {
	return "company,address,phone,website,rating,reviews,type_business,city,country\nAcme,Main St,,,4.5,10,store,Gotham,USA\n"
}
