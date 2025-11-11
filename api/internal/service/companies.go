package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/entity"
	"github.com/octobees/leads-generator/api/internal/repository"
)

// CompaniesService exposes read/write operations for the company catalogue.
type CompaniesService struct {
	repo repository.CompaniesRepository
}

// ErrInvalidCompanyID is returned when the provided company identifier cannot be parsed as UUID.
var (
	ErrInvalidCompanyID   = errors.New("invalid company_id")
	ErrEnrichmentNotFound = errors.New("company enrichment not found")
)

// CSVValidationError indicates that the provided CSV payload is invalid.
type CSVValidationError struct {
	Message string
}

// Error implements the error interface.
func (e CSVValidationError) Error() string {
	return e.Message
}

// UploadSummary reports how many rows were inserted or updated during import.
type UploadSummary struct {
	Inserted int `json:"inserted"`
	Updated  int `json:"updated"`
	Total    int `json:"total"`
}

// NewCompaniesService creates a new instance of CompaniesService.
func NewCompaniesService(repo repository.CompaniesRepository) *CompaniesService {
	return &CompaniesService{repo: repo}
}

// ListCompanies returns companies respecting pagination defaults.
func (s *CompaniesService) ListCompanies(ctx context.Context, filter dto.ListFilter) ([]entity.Company, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 20
	}
	if filter.PerPage > 100 {
		filter.PerPage = 100
	}
	if filter.Limit > 0 && filter.Limit < filter.PerPage {
		filter.PerPage = filter.Limit
	}
	return s.repo.List(ctx, filter)
}

// ImportCompaniesCSV ingests companies data from a CSV reader.
func (s *CompaniesService) ImportCompaniesCSV(ctx context.Context, r io.Reader) (UploadSummary, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return UploadSummary{}, CSVValidationError{Message: "csv file is empty"}
		}
		return UploadSummary{}, fmt.Errorf("read csv header: %w", err)
	}

	indexMap, valErr := buildHeaderIndex(header)
	if valErr != nil {
		return UploadSummary{}, valErr
	}

	var (
		records []repository.BulkUpsertCompanyInput
		rowNum  = 1
	)

	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return UploadSummary{}, fmt.Errorf("read csv row: %w", err)
		}

		rowNum++

		company := strings.TrimSpace(row[indexMap["company"]])
		address := strings.TrimSpace(row[indexMap["address"]])
		if company == "" || address == "" {
			continue
		}

		rating, parseErr := parseOptionalFloat(row[indexMap["rating"]])
		if parseErr != nil {
			return UploadSummary{}, CSVValidationError{Message: fmt.Sprintf("invalid rating value on row %d", rowNum)}
		}

		reviews, parseReviewsErr := parseOptionalInt(row[indexMap["reviews"]])
		if parseReviewsErr != nil {
			return UploadSummary{}, CSVValidationError{Message: fmt.Sprintf("invalid reviews value on row %d", rowNum)}
		}

		records = append(records, repository.BulkUpsertCompanyInput{
			Company:      company,
			Address:      address,
			Phone:        normalizeString(row[indexMap["phone"]]),
			Website:      normalizeString(row[indexMap["website"]]),
			Rating:       rating,
			Reviews:      reviews,
			TypeBusiness: normalizeString(row[indexMap["type_business"]]),
			City:         normalizeString(row[indexMap["city"]]),
			Country:      normalizeString(row[indexMap["country"]]),
		})
	}

	result, err := s.repo.BulkUpsertCompanies(ctx, records)
	if err != nil {
		return UploadSummary{}, err
	}

	return UploadSummary{
		Inserted: result.Inserted,
		Updated:  result.Updated,
		Total:    result.Total,
	}, nil
}

// UpsertCompany proxies to the repository to persist the record.
func (s *CompaniesService) UpsertCompany(ctx context.Context, company *entity.Company) error {
	return s.repo.Upsert(ctx, company)
}

// SaveEnrichment persists enrichment metadata for a company.
func (s *CompaniesService) SaveEnrichment(ctx context.Context, payload dto.EnrichResultRequest) error {
	companyID, err := uuid.Parse(strings.TrimSpace(payload.CompanyID))
	if err != nil {
		return ErrInvalidCompanyID
	}

	enrichment := &entity.CompanyEnrichment{
		CompanyID:      companyID,
		Emails:         normalizeStringSlice(payload.Emails, strings.ToLower),
		Phones:         normalizeStringSlice(payload.Phones, nil),
		Socials:        normalizeSocialLinks(payload.Socials),
		Address:        trimPointer(payload.Address),
		ContactFormURL: trimPointer(payload.ContactFormURL),
		AboutSummary:   trimPointer(payload.AboutSummary),
		Metadata:       buildEnrichmentMetadata(payload),
	}

	return s.repo.UpsertEnrichment(ctx, enrichment)
}

// GetEnrichment fetches enrichment metadata for a company.
func (s *CompaniesService) GetEnrichment(ctx context.Context, companyIDRaw string) (*entity.CompanyEnrichment, error) {
	companyID, err := uuid.Parse(strings.TrimSpace(companyIDRaw))
	if err != nil {
		return nil, ErrInvalidCompanyID
	}

	enrichment, err := s.repo.GetEnrichment(ctx, companyID)
	if err != nil {
		if errors.Is(err, repository.ErrEnrichmentNotFound) {
			return nil, ErrEnrichmentNotFound
		}
		return nil, err
	}
	return enrichment, nil
}

var requiredCSVHeaders = []string{"company", "address", "phone", "website", "rating", "reviews", "type_business", "city", "country"}

func buildHeaderIndex(header []string) (map[string]int, error) {
	index := make(map[string]int)
	for i, col := range header {
		index[strings.ToLower(strings.TrimSpace(col))] = i
	}

	missing := make([]string, 0)
	for _, required := range requiredCSVHeaders {
		if _, ok := index[required]; !ok {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		return nil, CSVValidationError{Message: fmt.Sprintf("missing required columns: %s", strings.Join(missing, ", "))}
	}
	return index, nil
}

func parseOptionalFloat(value string) (*float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func parseOptionalInt(value string) (*int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func normalizeString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func normalizeStringSlice(values []string, transform func(string) string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized := trimmed
		if transform != nil {
			normalized = transform(trimmed)
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeSocialLinks(values map[string][]string) map[string][]string {
	if len(values) == 0 {
		return nil
	}
	normalized := make(map[string][]string, len(values))
	for platform, links := range values {
		key := strings.TrimSpace(strings.ToLower(platform))
		if key == "" {
			continue
		}
		cleanLinks := normalizeStringSlice(links, nil)
		if len(cleanLinks) == 0 {
			continue
		}
		normalized[key] = cleanLinks
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func trimPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func buildEnrichmentMetadata(payload dto.EnrichResultRequest) map[string]any {
	meta := make(map[string]any)
	if website := strings.TrimSpace(payload.Website); website != "" {
		meta["website"] = website
	}
	if payload.PagesCrawled > 0 {
		meta["pages_crawled"] = payload.PagesCrawled
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}
