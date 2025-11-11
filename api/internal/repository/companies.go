package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/entity"
)

// CompaniesRepository describes persistence operations for companies.
type CompaniesRepository interface {
	Upsert(ctx context.Context, company *entity.Company) error
	List(ctx context.Context, filter dto.ListFilter) ([]entity.Company, error)
	BulkUpsertCompanies(ctx context.Context, records []BulkUpsertCompanyInput) (BulkUpsertResult, error)
	UpsertEnrichment(ctx context.Context, enrichment *entity.CompanyEnrichment) error
	GetEnrichment(ctx context.Context, companyID uuid.UUID) (*entity.CompanyEnrichment, error)
}

// ErrEnrichmentNotFound indicates there is no enrichment row for the given company.
var ErrEnrichmentNotFound = errors.New("company enrichment not found")

// BulkUpsertCompanyInput represents the minimal fields required for CSV ingestion.
type BulkUpsertCompanyInput struct {
	Company      string
	Phone        *string
	Website      *string
	Rating       *float64
	Reviews      *int
	TypeBusiness *string
	Address      string
	City         *string
	Country      *string
}

// BulkUpsertResult summarises the number of rows inserted or updated.
type BulkUpsertResult struct {
	Inserted int
	Updated  int
	Total    int
}

// PGXCompaniesRepository implements CompaniesRepository using pgx.
type PGXCompaniesRepository struct {
	pool pgxPool
}

// NewPGXCompaniesRepository wires a pgx backed repository.
func NewPGXCompaniesRepository(pool *pgxpool.Pool) *PGXCompaniesRepository {
	return &PGXCompaniesRepository{pool: pool}
}

var _ pgxPool = (*pgxpool.Pool)(nil)

// Upsert inserts or updates a company keyed by place_id.
func (r *PGXCompaniesRepository) Upsert(ctx context.Context, company *entity.Company) error {
	if company == nil {
		return fmt.Errorf("company payload is nil")
	}

	raw := company.Raw
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}

	var lng any
	if company.Longitude != nil {
		lng = *company.Longitude
	}
	var lat any
	if company.Latitude != nil {
		lat = *company.Latitude
	}

	query := `
        INSERT INTO companies (
            place_id,
            company,
            phone,
            website,
            rating,
            reviews,
            type_business,
            address,
            city,
            country,
            location,
            raw,
            scrape_run_id,
            scraped_at,
            updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
            CASE WHEN $11 IS NOT NULL AND $12 IS NOT NULL THEN
                ST_SetSRID(ST_MakePoint($11::float8, $12::float8), 4326)::geography
            ELSE NULL END,
            $13,
            $14,
            $15,
            NOW()
        )
        ON CONFLICT (place_id) DO UPDATE SET
            company = EXCLUDED.company,
            phone = EXCLUDED.phone,
            website = EXCLUDED.website,
            rating = EXCLUDED.rating,
            reviews = EXCLUDED.reviews,
            type_business = EXCLUDED.type_business,
            address = EXCLUDED.address,
            city = EXCLUDED.city,
            country = EXCLUDED.country,
            location = EXCLUDED.location,
            raw = EXCLUDED.raw,
            scrape_run_id = COALESCE(EXCLUDED.scrape_run_id, companies.scrape_run_id),
            scraped_at = COALESCE(EXCLUDED.scraped_at, companies.scraped_at),
            updated_at = NOW();
    `

	_, err := r.pool.Exec(ctx, query,
		company.PlaceID,
		company.Company,
		company.Phone,
		company.Website,
		company.Rating,
		company.Reviews,
		company.TypeBusiness,
		company.Address,
		company.City,
		company.Country,
		lng,
		lat,
		raw,
		company.ScrapeRunID,
		company.ScrapedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert company: %w", err)
	}

	return nil
}

const bulkUpsertSQL = `
        INSERT INTO companies (company, phone, website, rating, reviews, type_business, address, city, country, raw, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,NOW())
        ON CONFLICT (company, address) WHERE place_id IS NULL DO UPDATE SET
            phone = EXCLUDED.phone,
            website = EXCLUDED.website,
            rating = EXCLUDED.rating,
            reviews = EXCLUDED.reviews,
            type_business = EXCLUDED.type_business,
            city = EXCLUDED.city,
            country = EXCLUDED.country,
            updated_at = NOW()
        RETURNING xmax = 0;
    `

// BulkUpsertCompanies persists a batch of companies with idempotent semantics.
func (r *PGXCompaniesRepository) BulkUpsertCompanies(ctx context.Context, records []BulkUpsertCompanyInput) (BulkUpsertResult, error) {
	var result BulkUpsertResult
	if len(records) == 0 {
		return result, nil
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, fmt.Errorf("start bulk upsert tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, record := range records {
		rows, err := tx.Query(ctx, bulkUpsertSQL,
			record.Company,
			stringOrNil(record.Phone),
			stringOrNil(record.Website),
			floatOrNil(record.Rating),
			intOrNil(record.Reviews),
			stringOrNil(record.TypeBusiness),
			record.Address,
			stringOrNil(record.City),
			stringOrNil(record.Country),
			"{}",
		)
		if err != nil {
			return result, fmt.Errorf("bulk upsert company %q: %w", record.Company, err)
		}

		var inserted bool
		if rows.Next() {
			if scanErr := rows.Scan(&inserted); scanErr != nil {
				rows.Close()
				return result, fmt.Errorf("scan bulk upsert result: %w", scanErr)
			}
		} else {
			err := rows.Err()
			rows.Close()
			if err != nil {
				return result, fmt.Errorf("bulk upsert company %q: %w", record.Company, err)
			}
			return result, fmt.Errorf("bulk upsert company %q: no result returned", record.Company)
		}
		rows.Close()

		if inserted {
			result.Inserted++
		} else {
			result.Updated++
		}
		result.Total++
	}

	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit bulk upsert tx: %w", err)
	}

	return result, nil
}

// List retrieves companies matching the provided filter, sorted by rating then reviews.
func (r *PGXCompaniesRepository) List(ctx context.Context, filter dto.ListFilter) ([]entity.Company, error) {
	baseQuery := strings.Builder{}
	baseQuery.WriteString(`
        SELECT
            id,
            place_id,
            scrape_run_id,
            company,
            phone,
            website,
            rating,
            reviews,
            type_business,
            address,
            city,
            country,
            CASE WHEN location IS NOT NULL THEN ST_X(location::geometry) END AS longitude,
            CASE WHEN location IS NOT NULL THEN ST_Y(location::geometry) END AS latitude,
            raw,
            scraped_at,
            created_at,
            updated_at
        FROM companies
    `)

	var (
		clauses []string
		args    []any
		idx     = 1
	)

	if filter.Q != "" {
		pattern := fmt.Sprintf("%%%s%%", filter.Q)
		clauses = append(clauses, fmt.Sprintf("(company ILIKE $%d OR address ILIKE $%d)", idx, idx+1))
		args = append(args, pattern, pattern)
		idx += 2
	}
	if filter.TypeBusiness != "" {
		clauses = append(clauses, fmt.Sprintf("LOWER(type_business) = LOWER($%d)", idx))
		args = append(args, filter.TypeBusiness)
		idx++
	}
	if filter.City != "" {
		clauses = append(clauses, fmt.Sprintf("LOWER(city) = LOWER($%d)", idx))
		args = append(args, filter.City)
		idx++
	}
	if filter.Country != "" {
		clauses = append(clauses, fmt.Sprintf("LOWER(country) = LOWER($%d)", idx))
		args = append(args, filter.Country)
		idx++
	}
	if filter.MinRating != nil {
		clauses = append(clauses, fmt.Sprintf("rating >= $%d", idx))
		args = append(args, *filter.MinRating)
		idx++
	}
	switch strings.ToLower(filter.WebsiteStatus) {
	case "missing":
		clauses = append(clauses, "website IS NULL")
	case "available":
		clauses = append(clauses, "website IS NOT NULL")
	}
	if filter.LatestRunOnly && filter.UpdatedSince == nil && filter.ScrapeRunID == nil {
		runClauses := append([]string{}, clauses...)
		runClauses = append(runClauses, "scrape_run_id IS NOT NULL")

		runQuery := strings.Builder{}
		runQuery.WriteString("SELECT scrape_run_id, MAX(scraped_at) AS scraped_at FROM companies")
		if len(runClauses) > 0 {
			runQuery.WriteString(" WHERE ")
			runQuery.WriteString(strings.Join(runClauses, " AND "))
		}
		runQuery.WriteString(" GROUP BY scrape_run_id ORDER BY MAX(scraped_at) DESC LIMIT 1")

		var (
			latestRunID   sql.NullString
			latestScraped sql.NullTime
		)
		err := r.pool.QueryRow(ctx, runQuery.String(), args...).Scan(&latestRunID, &latestScraped)
		if err != nil {
			if err != pgx.ErrNoRows {
				return nil, fmt.Errorf("determine latest scrape run: %w", err)
			}
		} else if latestRunID.Valid {
			parsed, parseErr := uuid.Parse(latestRunID.String)
			if parseErr != nil {
				return nil, fmt.Errorf("parse latest scrape run id: %w", parseErr)
			}
			filter.ScrapeRunID = &parsed
		} else if latestScraped.Valid {
			ts := latestScraped.Time
			filter.UpdatedSince = &ts
		}
	}
	if filter.LatestRunOnly && filter.UpdatedSince == nil && filter.ScrapeRunID == nil {
		latestQuery := "SELECT MAX(updated_at) FROM companies"
		if len(clauses) > 0 {
			latestQuery += " WHERE " + strings.Join(clauses, " AND ")
		}
		var latest sql.NullTime
		if err := r.pool.QueryRow(ctx, latestQuery, args...).Scan(&latest); err != nil {
			return nil, fmt.Errorf("determine latest scrape window: %w", err)
		}
		if latest.Valid {
			ts := latest.Time
			filter.UpdatedSince = &ts
		} else {
			filter.LatestRunOnly = false
		}
	}
	if filter.ScrapeRunID != nil {
		clauses = append(clauses, fmt.Sprintf("scrape_run_id = $%d", idx))
		args = append(args, *filter.ScrapeRunID)
		idx++
	}
	if filter.UpdatedSince != nil {
		clauses = append(clauses, fmt.Sprintf("updated_at >= $%d", idx))
		args = append(args, *filter.UpdatedSince)
		idx++
	}

	if len(clauses) > 0 {
		baseQuery.WriteString(" WHERE ")
		baseQuery.WriteString(strings.Join(clauses, " AND "))
	}

	orderClause := "rating DESC NULLS LAST, reviews DESC NULLS LAST, company ASC"
	if strings.EqualFold(filter.Sort, "recent") || (filter.Sort == "" && filter.LatestRunOnly) {
		orderClause = "updated_at DESC, rating DESC NULLS LAST, company ASC"
	}
	baseQuery.WriteString(" ORDER BY ")
	baseQuery.WriteString(orderClause)

	if filter.Limit > 0 {
		baseQuery.WriteString(fmt.Sprintf(" LIMIT %d", filter.Limit))
	} else {
		page := filter.Page
		if page <= 0 {
			page = 1
		}
		perPage := filter.PerPage
		if perPage <= 0 {
			perPage = 20
		}
		if perPage > 100 {
			perPage = 100
		}
		offset := (page - 1) * perPage
		baseQuery.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1))
		args = append(args, perPage, offset)
	}

	rows, err := r.pool.Query(ctx, baseQuery.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list companies: %w", err)
	}
	defer rows.Close()

	return scanCompanies(rows)
}

// UpsertEnrichment stores or updates contact enrichment metadata for a company.
func (r *PGXCompaniesRepository) UpsertEnrichment(ctx context.Context, enrichment *entity.CompanyEnrichment) error {
	if enrichment == nil {
		return fmt.Errorf("enrichment payload is nil")
	}

	socials := enrichment.Socials
	if socials == nil {
		socials = map[string][]string{}
	}
	socialsJSON, err := json.Marshal(socials)
	if err != nil {
		return fmt.Errorf("marshal socials: %w", err)
	}

	metadata := enrichment.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	query := `
		INSERT INTO company_enrichments (
			company_id,
			emails,
			phones,
			socials,
			address,
			contact_form_url,
			about_summary,
			metadata,
			updated_at
		) VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, $8::jsonb, NOW())
		ON CONFLICT (company_id) DO UPDATE SET
			emails = EXCLUDED.emails,
			phones = EXCLUDED.phones,
			socials = EXCLUDED.socials,
			address = EXCLUDED.address,
			contact_form_url = EXCLUDED.contact_form_url,
			about_summary = EXCLUDED.about_summary,
			metadata = EXCLUDED.metadata,
			updated_at = NOW();
	`

	_, err = r.pool.Exec(ctx, query,
		enrichment.CompanyID,
		stringSliceOrEmpty(enrichment.Emails),
		stringSliceOrEmpty(enrichment.Phones),
		string(socialsJSON),
		enrichment.Address,
		enrichment.ContactFormURL,
		enrichment.AboutSummary,
		string(metadataJSON),
	)
	if err != nil {
		return fmt.Errorf("upsert enrichment: %w", err)
	}

	return nil
}

func stringSliceOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// GetEnrichment returns the enrichment metadata for a given company.
func (r *PGXCompaniesRepository) GetEnrichment(ctx context.Context, companyID uuid.UUID) (*entity.CompanyEnrichment, error) {
	query := `
		SELECT
			company_id,
			emails,
			phones,
			socials,
			address,
			contact_form_url,
			about_summary,
			metadata,
			created_at,
			updated_at
		FROM company_enrichments
		WHERE company_id = $1
	`

	var (
		record          entity.CompanyEnrichment
		emails          []string
		phones          []string
		socialsJSON     []byte
		metadataJSON    []byte
		address         sql.NullString
		contactForm     sql.NullString
		aboutSummary    sql.NullString
	)

	err := r.pool.QueryRow(ctx, query, companyID).Scan(
		&record.CompanyID,
		&emails,
		&phones,
		&socialsJSON,
		&address,
		&contactForm,
		&aboutSummary,
		&metadataJSON,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEnrichmentNotFound
		}
		return nil, fmt.Errorf("fetch enrichment: %w", err)
	}

	if len(emails) > 0 {
		record.Emails = append([]string(nil), emails...)
	}
	if len(phones) > 0 {
		record.Phones = append([]string(nil), phones...)
	}
	if len(socialsJSON) > 0 {
		if err := json.Unmarshal(socialsJSON, &record.Socials); err != nil {
			return nil, fmt.Errorf("unmarshal socials: %w", err)
		}
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &record.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	record.Address = nullStringToPtr(address)
	record.ContactFormURL = nullStringToPtr(contactForm)
	record.AboutSummary = nullStringToPtr(aboutSummary)

	return &record, nil
}

func nullStringToPtr(value sql.NullString) *string {
	if value.Valid {
		val := value.String
		return &val
	}
	return nil
}

func scanCompanies(rows pgx.Rows) ([]entity.Company, error) {
	var companies []entity.Company
	for rows.Next() {
		var (
			c            entity.Company
			placeID      sql.NullString
			scrapeRunID  sql.NullString
			phone        sql.NullString
			website      sql.NullString
			rating       sql.NullFloat64
			reviews      sql.NullInt64
			typeBusiness sql.NullString
			address      sql.NullString
			city         sql.NullString
			country      sql.NullString
			longitude    sql.NullFloat64
			latitude     sql.NullFloat64
			raw          []byte
			scrapedAt    sql.NullTime
		)

		err := rows.Scan(
			&c.ID,
			&placeID,
			&scrapeRunID,
			&c.Company,
			&phone,
			&website,
			&rating,
			&reviews,
			&typeBusiness,
			&address,
			&city,
			&country,
			&longitude,
			&latitude,
			&raw,
			&scrapedAt,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan company: %w", err)
		}

		if placeID.Valid {
			val := placeID.String
			c.PlaceID = &val
		}
		if scrapeRunID.Valid {
			parsed, err := uuid.Parse(scrapeRunID.String)
			if err != nil {
				return nil, fmt.Errorf("parse scrape_run_id: %w", err)
			}
			c.ScrapeRunID = &parsed
		}
		if phone.Valid {
			val := phone.String
			c.Phone = &val
		}
		if website.Valid {
			val := website.String
			c.Website = &val
		}
		if rating.Valid {
			val := rating.Float64
			c.Rating = &val
		}
		if reviews.Valid {
			cast := int(reviews.Int64)
			c.Reviews = &cast
		}
		if typeBusiness.Valid {
			val := typeBusiness.String
			c.TypeBusiness = &val
		}
		if address.Valid {
			val := address.String
			c.Address = &val
		}
		if city.Valid {
			val := city.String
			c.City = &val
		}
		if country.Valid {
			val := country.String
			c.Country = &val
		}
		if longitude.Valid {
			val := longitude.Float64
			c.Longitude = &val
		}
		if latitude.Valid {
			val := latitude.Float64
			c.Latitude = &val
		}

		if len(raw) > 0 {
			c.Raw = json.RawMessage(raw)
		} else {
			c.Raw = json.RawMessage([]byte("{}"))
		}
		if scrapedAt.Valid {
			ts := scrapedAt.Time
			c.ScrapedAt = &ts
		}

		companies = append(companies, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate companies: %w", err)
	}
	return companies, nil
}

func stringOrNil(value *string) any {
	if value == nil {
		return nil
	}
	if *value == "" {
		return nil
	}
	return *value
}

func floatOrNil(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func intOrNil(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
