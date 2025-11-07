package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/service"
)

// CompaniesHandler exposes company catalogue endpoints.
type CompaniesHandler struct {
	service *service.CompaniesService
}

// NewCompaniesHandler creates a new handler instance.
func NewCompaniesHandler(service *service.CompaniesService) *CompaniesHandler {
	return &CompaniesHandler{service: service}
}

// List handles GET /companies requests.
func (h *CompaniesHandler) List(c echo.Context) error {
	return h.listInternal(c, true)
}

// ListAdmin handles GET /admin/companies requests.
func (h *CompaniesHandler) ListAdmin(c echo.Context) error {
	return h.listInternal(c, false)
}

func (h *CompaniesHandler) listInternal(c echo.Context, latestOnly bool) error {
	filter := dto.ListFilter{
		Q:            strings.TrimSpace(c.QueryParam("q")),
		TypeBusiness: strings.TrimSpace(c.QueryParam("type_business")),
		City:         strings.TrimSpace(c.QueryParam("city")),
		Country:      strings.TrimSpace(c.QueryParam("country")),
		Sort:         strings.TrimSpace(c.QueryParam("sort")),
		Page:         parseIntDefault(c.QueryParam("page"), 1),
		PerPage:      parseIntDefault(c.QueryParam("per_page"), 20),
	}

	if minRatingStr := strings.TrimSpace(c.QueryParam("min_rating")); minRatingStr != "" {
		if minRating, err := strconv.ParseFloat(minRatingStr, 64); err == nil {
			filter.MinRating = &minRating
		}
	}

	if latestOnly {
		filter.LatestRunOnly = true
		if filter.Sort == "" {
			filter.Sort = "recent"
		}
	}

	if runIDParam := strings.TrimSpace(c.QueryParam("scrape_run_id")); runIDParam != "" {
		parsed, err := uuid.Parse(runIDParam)
		if err != nil {
			return Error(c, http.StatusBadRequest, "invalid scrape_run_id")
		}
		filter.ScrapeRunID = &parsed
	}

	if updatedSinceStr := strings.TrimSpace(c.QueryParam("updated_since")); updatedSinceStr != "" {
		parsed, err := time.Parse(time.RFC3339, updatedSinceStr)
		if err != nil {
			return Error(c, http.StatusBadRequest, "invalid updated_since (use RFC3339)")
		}
		filter.UpdatedSince = &parsed
	}

	companies, err := h.service.ListCompanies(c.Request().Context(), filter)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "failed to list companies")
	}

	return Success(c, http.StatusOK, "companies retrieved", companies)
}

func parseIntDefault(input string, fallback int) int {
	if input == "" {
		return fallback
	}
	if value, err := strconv.Atoi(input); err == nil {
		return value
	}
	return fallback
}
