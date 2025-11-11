package handler

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/dto"
	middlewarepkg "github.com/octobees/leads-generator/api/internal/middleware"
)

// EnrichWorkerHandler forwards enrichment jobs to the worker service.
type EnrichWorkerHandler struct {
	worker WorkerPoster
}

// NewEnrichWorkerHandler constructs an enrichment job handler backed by HTTP client.
func NewEnrichWorkerHandler(client *http.Client, workerBaseURL string) *EnrichWorkerHandler {
	return &EnrichWorkerHandler{worker: NewWorkerClient(client, workerBaseURL)}
}

// NewEnrichWorkerHandlerWithWorker injects a custom worker client.
func NewEnrichWorkerHandlerWithWorker(worker WorkerPoster) *EnrichWorkerHandler {
	return &EnrichWorkerHandler{worker: worker}
}

// Enqueue validates the request and forwards it to the worker enrichment endpoint.
func (h *EnrichWorkerHandler) Enqueue(c echo.Context) error {
	var req dto.EnrichJobRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, http.StatusBadRequest, "invalid payload")
	}

	req.CompanyID = strings.TrimSpace(req.CompanyID)
	req.Website = strings.TrimSpace(req.Website)
	if req.CompanyID == "" || req.Website == "" {
		return Error(c, http.StatusBadRequest, "company_id and website are required")
	}

	ctx := c.Request().Context()
	data, err := h.worker.PostJSON(ctx, "/enrich", map[string]string{
		"company_id": req.CompanyID,
		"website":    req.Website,
	}, middlewarepkg.RequestIDFromContext(c))
	if err != nil {
		return Error(c, http.StatusBadGateway, err.Error())
	}
	if data == nil {
		data = map[string]any{"status": "queued"}
	}
	return Success(c, http.StatusOK, "enrichment job queued", data)
}
