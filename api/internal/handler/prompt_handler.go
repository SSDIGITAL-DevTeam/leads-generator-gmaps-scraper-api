package handler

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/service"
	middlewarepkg "github.com/octobees/leads-generator/api/internal/middleware"
)

// PromptSearchHandler accepts free-form prompts and forwards derived jobs to the worker.
type PromptSearchHandler struct {
	worker  WorkerPoster
	service *service.PromptService
}

// NewPromptSearchHandler wires the handler.
func NewPromptSearchHandler(worker WorkerPoster, svc *service.PromptService) *PromptSearchHandler {
	return &PromptSearchHandler{worker: worker, service: svc}
}

// Enqueue parses prompts and calls worker scrape endpoint.
func (h *PromptSearchHandler) Enqueue(c echo.Context) error {
	var req dto.PromptSearchRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, http.StatusBadRequest, "invalid payload")
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		return Error(c, http.StatusBadRequest, "prompt is required")
	}

	result, err := h.service.Parse(req)
	if err != nil {
		return Error(c, http.StatusBadRequest, err.Error())
	}

	payload := map[string]any{
		"type_business": result.TypeBusiness,
		"city":          result.City,
		"country":       result.Country,
	}
	if result.MinRating > 0 {
		payload["min_rating"] = result.MinRating
	}

	ctx := c.Request().Context()
	data, err := h.worker.PostJSON(ctx, "/scrape", payload, middlewarepkg.RequestIDFromContext(c))
	if err != nil {
		return Error(c, http.StatusBadGateway, err.Error())
	}
	if data == nil {
		data = map[string]any{"status": "queued"}
	}

	resp := dto.PromptSearchResponse{
		Prompt:          req.Prompt,
		TypeBusiness:    result.TypeBusiness,
		City:            result.City,
		Country:         result.Country,
		MinRating:       result.MinRating,
		Limit:           result.Limit,
		RequireNoWebsite: result.RequireNoWebsite,
	}

	queryParams := map[string]any{
		"type_business": result.TypeBusiness,
		"city":          result.City,
		"country":      result.Country,
	}
	if result.MinRating > 0 {
		queryParams["min_rating"] = result.MinRating
	}
	if result.Limit > 0 {
		queryParams["limit"] = result.Limit
	}
	if result.RequireNoWebsite {
		queryParams["website"] = "missing"
	}

	return Success(c, http.StatusOK, "prompt job queued", map[string]any{
		"job":          data,
		"query":        resp,
		"query_params": queryParams,
	})
}
