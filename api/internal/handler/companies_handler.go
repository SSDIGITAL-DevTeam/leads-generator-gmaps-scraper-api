package handler

import (
	"net/http"
	"strconv"
	"strings"

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
	return h.listInternal(c)
}

// ListAdmin handles GET /admin/companies requests.
func (h *CompaniesHandler) ListAdmin(c echo.Context) error {
	return h.listInternal(c)
}

func (h *CompaniesHandler) listInternal(c echo.Context) error {
	filter := dto.ListFilter{
		Q:            strings.TrimSpace(c.QueryParam("q")),
		TypeBusiness: strings.TrimSpace(c.QueryParam("type_business")),
		City:         strings.TrimSpace(c.QueryParam("city")),
		Country:      strings.TrimSpace(c.QueryParam("country")),
		Page:         parseIntDefault(c.QueryParam("page"), 1),
		PerPage:      parseIntDefault(c.QueryParam("per_page"), 20),
	}

	if minRatingStr := strings.TrimSpace(c.QueryParam("min_rating")); minRatingStr != "" {
		if minRating, err := strconv.ParseFloat(minRatingStr, 64); err == nil {
			filter.MinRating = &minRating
		}
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
