package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/entity"
	"github.com/octobees/leads-generator/api/internal/service"
	"github.com/octobees/leads-generator/api/internal/service/scoring"
)

// EnrichHandler receives website enrichment payloads from the worker service.
type EnrichHandler struct {
	companiesService *service.CompaniesService
}

// NewEnrichHandler wires a new EnrichHandler instance.
func NewEnrichHandler(companiesService *service.CompaniesService) *EnrichHandler {
	return &EnrichHandler{companiesService: companiesService}
}

// SaveResult persists the POSTed enrichment payload.
func (h *EnrichHandler) SaveResult(c echo.Context) error {
	var payload dto.EnrichResultRequest
	if err := c.Bind(&payload); err != nil {
		return Error(c, http.StatusBadRequest, "invalid JSON payload")
	}
	if payload.CompanyID == "" {
		return Error(c, http.StatusBadRequest, "company_id is required")
	}

	if err := h.companiesService.SaveEnrichment(c.Request().Context(), payload); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCompanyID):
			return Error(c, http.StatusBadRequest, "invalid company_id")
		default:
			return Error(c, http.StatusInternalServerError, "failed to persist enrichment")
		}
	}

	return Success(c, http.StatusOK, "enrichment stored", map[string]any{"success": true})
}

// GetResult retrieves the enrichment payload for a company.
func (h *EnrichHandler) GetResult(c echo.Context) error {
	companyID := c.Param("company_id")
	if companyID == "" {
		return Error(c, http.StatusBadRequest, "company_id is required")
	}

	result, err := h.companiesService.GetEnrichment(c.Request().Context(), companyID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCompanyID):
			return Error(c, http.StatusBadRequest, "invalid company_id")
		case errors.Is(err, service.ErrEnrichmentNotFound):
			return Error(c, http.StatusNotFound, "enrichment not found")
		default:
			return Error(c, http.StatusInternalServerError, "failed to fetch enrichment")
		}
	}

	features := buildLeadFeatures(result)
	score := scoring.ComputeScore(features)

	payload := map[string]any{
		"enrichment": result,
		"score":      score,
	}

	return Success(c, http.StatusOK, "ok", payload)
}

func buildLeadFeatures(enrichment *entity.CompanyEnrichment) scoring.LeadFeatures {
	if enrichment == nil {
		return scoring.LeadFeatures{}
	}

	socials := flattenSocials(enrichment.Socials)
	address := derefString(enrichment.Address)
	website := metadataString(enrichment.Metadata, "website")

	hasContactForm := hasPointerValue(enrichment.ContactFormURL)
	hasContactPage := hasContactForm
	if !hasContactPage {
		if metadataString(enrichment.Metadata, "contact_page_url") != "" {
			hasContactPage = true
		} else if flag, ok := metadataBool(enrichment.Metadata, "has_contact_page"); ok {
			hasContactPage = flag
		}
	}

	hasHTTPS := metadataBoolDefault(enrichment.Metadata, "https_enabled")
	if !hasHTTPS && website != "" {
		hasHTTPS = strings.HasPrefix(strings.ToLower(website), "https://")
	}

	return scoring.LeadFeatures{
		Emails:         enrichment.Emails,
		Phones:         enrichment.Phones,
		Socials:        socials,
		HasHTTPS:       hasHTTPS,
		HasContactPage: hasContactPage,
		HasAboutPage:   hasPointerValue(enrichment.AboutSummary),
		HasContactForm: hasContactForm,
		Address:        address,
		Website:        website,
	}
}

func flattenSocials(values map[string][]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for platform, links := range values {
		value := firstNonEmpty(links)
		if value == "" {
			continue
		}
		result[platform] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func firstNonEmpty(values []string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func hasPointerValue(value *string) bool {
	return derefString(value) != ""
}

func metadataString(meta map[string]any, key string) string {
	if len(meta) == 0 {
		return ""
	}
	if raw, ok := meta[key]; ok {
		if str, ok := raw.(string); ok {
			return strings.TrimSpace(str)
		}
	}
	return ""
}

func metadataBool(meta map[string]any, key string) (bool, bool) {
	if len(meta) == 0 {
		return false, false
	}
	raw, ok := meta[key]
	if !ok {
		return false, false
	}
	flag, valid := raw.(bool)
	return flag, valid
}

func metadataBoolDefault(meta map[string]any, key string) bool {
	if flag, ok := metadataBool(meta, key); ok {
		return flag
	}
	return false
}
