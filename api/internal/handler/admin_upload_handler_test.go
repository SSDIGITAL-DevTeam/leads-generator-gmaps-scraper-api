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

func (s *stubCompaniesRepository) UpsertEnrichment(ctx context.Context, enrichment *entity.CompanyEnrichment) error {
	return nil
}

func newAdminUploadHandler(repo repository.CompaniesRepository) *AdminUploadHandler {
	service := service.NewCompaniesService(repo)
	return NewAdminUploadHandler(service)
}

func TestAdminUploadHandler_UploadCSV(t *testing.T) {
	tests := []struct {
		name       string
		request    func(t *testing.T) (*http.Request, *httptest.ResponseRecorder)
		repo       repository.CompaniesRepository
		wantCode   int
	}{
		{
			name: "missing file",
			request: func(t *testing.T) (*http.Request, *httptest.ResponseRecorder) {
				req := httptest.NewRequest(http.MethodPost, "/admin/upload-csv", nil)
				rec := httptest.NewRecorder()
				return req, rec
			},
			repo:     &stubCompaniesRepository{},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "invalid csv",
			request: func(t *testing.T) (*http.Request, *httptest.ResponseRecorder) {
				return multipartRequest(t, "file", "test.csv", "company,address\nAcme,Main St\n")
			},
			repo:     &stubCompaniesRepository{},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "repository error",
			request: func(t *testing.T) (*http.Request, *httptest.ResponseRecorder) {
				return multipartRequest(t, "file", "test.csv", validCSV())
			},
			repo: &stubCompaniesRepository{
				bulk: func(ctx context.Context, records []repository.BulkUpsertCompanyInput) (repository.BulkUpsertResult, error) {
					return repository.BulkUpsertResult{}, context.DeadlineExceeded
				},
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "success",
			request: func(t *testing.T) (*http.Request, *httptest.ResponseRecorder) {
				return multipartRequest(t, "file", "test.csv", validCSV())
			},
			repo: &stubCompaniesRepository{
				bulk: func(ctx context.Context, records []repository.BulkUpsertCompanyInput) (repository.BulkUpsertResult, error) {
					if len(records) != 1 {
						t.Fatalf("expected 1 record, got %d", len(records))
					}
					return repository.BulkUpsertResult{Inserted: 1, Total: 1}, nil
				},
			},
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req, rec := tt.request(t)
			c := e.NewContext(req, rec)

			handler := newAdminUploadHandler(tt.repo)
			_ = handler.UploadCSV(c)
			if rec.Code != tt.wantCode {
				t.Fatalf("expected status %d, got %d", tt.wantCode, rec.Code)
			}
		})
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
