package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/service"
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

	return Success(c, http.StatusOK, "enrichment stored", map[string]any{"company_id": payload.CompanyID})
}
