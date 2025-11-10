package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/octobees/leads-generator/api/internal/entity"
)

type stubCompanyRows struct {
	called bool
}

func (s *stubCompanyRows) Close()                                       {}
func (s *stubCompanyRows) Err() error                                   { return nil }
func (s *stubCompanyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (s *stubCompanyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (s *stubCompanyRows) Next() bool {
	if s.called {
		return false
	}
	s.called = true
	return true
}

func (s *stubCompanyRows) Scan(dest ...any) error {
	if !s.called {
		return errors.New("scan called before next")
	}
	id := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	runID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	created := time.Now()
	updated := created
	placeID := sql.NullString{String: "place-123", Valid: true}
	runIDVal := sql.NullString{String: runID.String(), Valid: true}
	phone := sql.NullString{String: "+123456", Valid: true}
	website := sql.NullString{String: "https://example.com", Valid: true}
	rating := sql.NullFloat64{Float64: 4.5, Valid: true}
	reviews := sql.NullInt64{Int64: 100, Valid: true}
	typeBusiness := sql.NullString{String: "store", Valid: true}
	address := sql.NullString{String: "Main St", Valid: true}
	city := sql.NullString{String: "Gotham", Valid: true}
	country := sql.NullString{String: "USA", Valid: true}
	lng := sql.NullFloat64{Float64: 10.0, Valid: true}
	lat := sql.NullFloat64{Float64: 20.0, Valid: true}
	raw := []byte(`{"foo":"bar"}`)
	scrapedAt := sql.NullTime{Time: created, Valid: true}

	*dest[0].(*uuid.UUID) = id
	*dest[1].(*sql.NullString) = placeID
	*dest[2].(*sql.NullString) = runIDVal
	*dest[3].(*string) = "Acme"
	*dest[4].(*sql.NullString) = phone
	*dest[5].(*sql.NullString) = website
	*dest[6].(*sql.NullFloat64) = rating
	*dest[7].(*sql.NullInt64) = reviews
	*dest[8].(*sql.NullString) = typeBusiness
	*dest[9].(*sql.NullString) = address
	*dest[10].(*sql.NullString) = city
	*dest[11].(*sql.NullString) = country
	*dest[12].(*sql.NullFloat64) = lng
	*dest[13].(*sql.NullFloat64) = lat
	*dest[14].(*[]byte) = raw
	*dest[15].(*sql.NullTime) = scrapedAt
	*dest[16].(*time.Time) = created
	*dest[17].(*time.Time) = updated
	return nil
}

func (s *stubCompanyRows) Values() ([]any, error) { return nil, nil }
func (s *stubCompanyRows) RawValues() [][]byte    { return nil }
func (s *stubCompanyRows) Conn() *pgx.Conn        { return nil }

func TestPGXCompaniesRepository_UpsertValidation(t *testing.T) {
	repo := &PGXCompaniesRepository{}
	if err := repo.Upsert(context.Background(), nil); err == nil {
		t.Fatalf("expected error for nil company")
	}
}

func TestPGXCompaniesRepository_BulkUpsertEmpty(t *testing.T) {
	repo := &PGXCompaniesRepository{}
	res, err := repo.BulkUpsertCompanies(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total != 0 {
		t.Fatalf("expected zero summary, got %+v", res)
	}
}

func TestScanCompanies(t *testing.T) {
	rows, err := scanCompanies(&stubCompanyRows{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 company, got %d", len(rows))
	}
	company := rows[0]
	if company.Company != "Acme" || company.PlaceID == nil || *company.PlaceID != "place-123" {
		t.Fatalf("unexpected company: %+v", company)
	}
	if company.ScrapeRunID == nil || company.ScrapeRunID.String() != "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" {
		t.Fatalf("expected scrape_run_id set, got %+v", company.ScrapeRunID)
	}
	if company.ScrapedAt == nil {
		t.Fatalf("expected scraped_at set")
	}
	if company.Longitude == nil || *company.Longitude != 10.0 {
		t.Fatalf("expected longitude to be set")
	}
	if company.Raw == nil || string(company.Raw) != "{\"foo\":\"bar\"}" {
		t.Fatalf("unexpected raw payload: %s", string(company.Raw))
	}
}

func TestHelperConversions(t *testing.T) {
	if stringOrNil(nil) != nil {
		t.Fatalf("expected nil when pointer nil")
	}
	value := "hello"
	if stringOrNil(&value) != "hello" {
		t.Fatalf("expected string value")
	}

	if floatOrNil(nil) != nil {
		t.Fatalf("expected nil for nil float pointer")
	}
	f := 3.14
	if floatOrNil(&f) != f {
		t.Fatalf("expected float value")
	}

	if intOrNil(nil) != nil {
		t.Fatalf("expected nil for nil int pointer")
	}
	i := 42
	if intOrNil(&i) != i {
		t.Fatalf("expected int value")
	}

	if res := stringSliceOrEmpty(nil); len(res) != 0 {
		t.Fatalf("expected empty slice when input nil")
	}
	if res := stringSliceOrEmpty([]string{"a"}); len(res) != 1 || res[0] != "a" {
		t.Fatalf("expected matching slice, got %+v", res)
	}
}

func TestPGXCompaniesRepository_UpsertEnrichment_Validation(t *testing.T) {
	repo := &PGXCompaniesRepository{}
	if err := repo.UpsertEnrichment(context.Background(), nil); err == nil {
		t.Fatalf("expected error for nil enrichment")
	}
}

func TestPGXCompaniesRepository_UpsertEnrichment_Success(t *testing.T) {
	companyID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	address := "Main St"
	contact := "https://acme.com/contact"
	enrichment := &entity.CompanyEnrichment{
		CompanyID:      companyID,
		Emails:         []string{"info@example.com"},
		Phones:         []string{"+123"},
		Socials:        map[string][]string{"linkedin": {"https://linkedin.com/company/acme"}},
		Address:        &address,
		ContactFormURL: &contact,
		Metadata:       map[string]any{"website": "https://acme.com"},
	}

	called := false
	repo := &PGXCompaniesRepository{pool: &stubPool{
		execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			called = true
			if len(args) != 8 {
				t.Fatalf("expected 8 args, got %d", len(args))
			}
			if args[0] != companyID {
				t.Fatalf("expected company id arg, got %v", args[0])
			}
			if addr, _ := args[4].(*string); addr == nil || *addr != "Main St" {
				t.Fatalf("expected address arg")
			}
			return pgconn.CommandTag{}, nil
		},
	}}

	if err := repo.UpsertEnrichment(context.Background(), enrichment); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected exec to be called")
	}
}
