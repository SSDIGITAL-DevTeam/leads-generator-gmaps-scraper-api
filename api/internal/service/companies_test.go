package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/entity"
	"github.com/octobees/leads-generator/api/internal/repository"
)

type mockCompaniesRepository struct {
	list   func(ctx context.Context, filter dto.ListFilter) ([]entity.Company, error)
	bulk   func(ctx context.Context, records []repository.BulkUpsertCompanyInput) (repository.BulkUpsertResult, error)
	upsert func(ctx context.Context, company *entity.Company) error
}

func (m *mockCompaniesRepository) List(ctx context.Context, filter dto.ListFilter) ([]entity.Company, error) {
	if m.list != nil {
		return m.list(ctx, filter)
	}
	return nil, errors.New("list not implemented")
}

func (m *mockCompaniesRepository) BulkUpsertCompanies(ctx context.Context, records []repository.BulkUpsertCompanyInput) (repository.BulkUpsertResult, error) {
	if m.bulk != nil {
		return m.bulk(ctx, records)
	}
	return repository.BulkUpsertResult{}, errors.New("bulk not implemented")
}

func (m *mockCompaniesRepository) Upsert(ctx context.Context, company *entity.Company) error {
	if m.upsert != nil {
		return m.upsert(ctx, company)
	}
	return errors.New("upsert not implemented")
}

func TestCompaniesService_ListCompanies_AppliesDefaults(t *testing.T) {
	received := dto.ListFilter{}
	repo := &mockCompaniesRepository{
		list: func(ctx context.Context, filter dto.ListFilter) ([]entity.Company, error) {
			received = filter
			return []entity.Company{{Company: "Acme"}}, nil
		},
	}

	service := NewCompaniesService(repo)
	filter := dto.ListFilter{Page: -1, PerPage: 0}
	companies, err := service.ListCompanies(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(companies) != 1 {
		t.Fatalf("expected 1 company, got %d", len(companies))
	}
	if received.Page != 1 {
		t.Fatalf("expected page default 1, got %d", received.Page)
	}
	if received.PerPage != 20 {
		t.Fatalf("expected per_page default 20, got %d", received.PerPage)
	}
}

func TestCompaniesService_ListCompanies_CapsPerPage(t *testing.T) {
	repo := &mockCompaniesRepository{
		list: func(ctx context.Context, filter dto.ListFilter) ([]entity.Company, error) {
			if filter.PerPage != 100 {
				t.Fatalf("expected per_page capped at 100, got %d", filter.PerPage)
			}
			return nil, nil
		},
	}
	service := NewCompaniesService(repo)
	service.ListCompanies(context.Background(), dto.ListFilter{PerPage: 500})
}

func TestCompaniesService_ImportCompaniesCSV(t *testing.T) {
	tests := map[string]struct {
		csv         string
		mock        *mockCompaniesRepository
		expectError string
	}{
		"empty file": {
			csv:         ``,
			mock:        &mockCompaniesRepository{},
			expectError: "csv file is empty",
		},
		"missing headers": {
			csv:         "company,address\nAcme,Main St",
			mock:        &mockCompaniesRepository{},
			expectError: "missing required columns",
		},
		"invalid rating": {
			csv: "company,address,phone,website,rating,reviews,type_business,city,country\n" +
				"Acme,Main St,,,bad,10,store,Gotham,USA\n",
			mock:        &mockCompaniesRepository{},
			expectError: "invalid rating value",
		},
		"success": {
			csv: "company,address,phone,website,rating,reviews,type_business,city,country\n" +
				"Acme,Main St,123456,https://acme.com,4.5,10,store,Gotham,USA\n",
			mock: &mockCompaniesRepository{
				bulk: func(ctx context.Context, records []repository.BulkUpsertCompanyInput) (repository.BulkUpsertResult, error) {
					if len(records) != 1 {
						t.Fatalf("expected 1 record, got %d", len(records))
					}
					rec := records[0]
					if rec.Company != "Acme" || rec.Address != "Main St" {
						t.Fatalf("unexpected record payload: %+v", rec)
					}
					return repository.BulkUpsertResult{Inserted: 1, Updated: 0, Total: 1}, nil
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			service := NewCompaniesService(tt.mock)
			summary, err := service.ImportCompaniesCSV(context.Background(), strings.NewReader(tt.csv))
			if tt.expectError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectError) {
					t.Fatalf("expected error containing %q, got %v", tt.expectError, err)
				}
				if (summary != UploadSummary{}) {
					t.Fatalf("expected zero summary on error, got %+v", summary)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if summary.Inserted != 1 || summary.Total != 1 {
				t.Fatalf("unexpected summary: %+v", summary)
			}
		})
	}
}

func TestCompaniesService_UpsertCompany(t *testing.T) {
	called := false
	repo := &mockCompaniesRepository{
		upsert: func(ctx context.Context, company *entity.Company) error {
			called = true
			if company.Company != "Acme" {
				t.Fatalf("unexpected company payload: %+v", company)
			}
			return nil
		},
	}

	service := NewCompaniesService(repo)
	err := service.UpsertCompany(context.Background(), &entity.Company{Company: "Acme"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected repository to be invoked")
	}
}

func TestBuildHeaderIndex(t *testing.T) {
	header := []string{"Company", "Address", "Phone", "Website", "Rating", "Reviews", "Type_Business", "City", "Country"}
	index, err := buildHeaderIndex(header)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if index["company"] != 0 || index["address"] != 1 {
		t.Fatalf("header index not built correctly: %+v", index)
	}

	_, err = buildHeaderIndex([]string{"company", "address"})
	if err == nil {
		t.Fatalf("expected error for missing headers")
	}
}

func TestParseOptionalFloat(t *testing.T) {
	val, err := parseOptionalFloat("4.2")
	if err != nil || val == nil || *val != 4.2 {
		t.Fatalf("expected 4.2, got %v (err=%v)", val, err)
	}
	val, err = parseOptionalFloat("")
	if err != nil || val != nil {
		t.Fatalf("expected nil for empty input")
	}
	if _, err = parseOptionalFloat("bad"); err == nil {
		t.Fatalf("expected parse error for invalid float")
	}
}

func TestParseOptionalInt(t *testing.T) {
	val, err := parseOptionalInt("7")
	if err != nil || val == nil || *val != 7 {
		t.Fatalf("expected 7, got %v (err=%v)", val, err)
	}
	val, err = parseOptionalInt("")
	if err != nil || val != nil {
		t.Fatalf("expected nil for empty input")
	}
	if _, err = parseOptionalInt("bad"); err == nil {
		t.Fatalf("expected parse error for invalid int")
	}
}

func TestNormalizeString(t *testing.T) {
	if got := normalizeString("  hello "); got == nil || *got != "hello" {
		t.Fatalf("expected trimmed string, got %v", got)
	}
	if got := normalizeString("   "); got != nil {
		t.Fatalf("expected nil for whitespace string")
	}
}
