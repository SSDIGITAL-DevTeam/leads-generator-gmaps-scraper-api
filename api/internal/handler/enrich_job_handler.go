package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/dto"
	middlewarepkg "github.com/octobees/leads-generator/api/internal/middleware"

	"google.golang.org/api/idtoken"
)

// EnrichWorkerHandler forwards enrichment jobs to the worker service.
type EnrichWorkerHandler struct {
	client        *http.Client
	workerBaseURL string
}

// NewEnrichWorkerHandler constructs an enrichment job handler backed by HTTP client.
func NewEnrichWorkerHandler(client *http.Client, workerBaseURL string) *EnrichWorkerHandler {
	if workerBaseURL == "" {
		panic("workerBaseURL must not be empty")
	}

	workerBaseURL = strings.TrimRight(workerBaseURL, "/")

	if client == nil {
		idc, err := idtoken.NewClient(context.Background(), workerBaseURL)
		if err != nil {
			client = &http.Client{Timeout: 10 * time.Second}
		} else {
			client = idc
		}
	}

	return &EnrichWorkerHandler{
		client:        client,
		workerBaseURL: workerBaseURL,
	}
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

	body, err := json.Marshal(map[string]string{
		"company_id": req.CompanyID,
		"website":    req.Website,
	})
	if err != nil {
		return Error(c, http.StatusInternalServerError, "failed to marshal payload")
	}

	ctx := c.Request().Context()
	workerURL := h.workerBaseURL + "/enrich"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, workerURL, bytes.NewReader(body))
	if err != nil {
		return Error(c, http.StatusInternalServerError, "failed to create worker request")
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if rid := middlewarepkg.RequestIDFromContext(c); rid != "" {
		httpReq.Header.Set("X-Request-ID", rid)
	}

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return Error(c, http.StatusBadGateway, fmt.Sprintf("worker request failed: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return Error(c, http.StatusBadGateway, extractWorkerError(resp.Body))
	}

	var workerResp struct {
		Data  map[string]any `json:"data"`
		Error string         `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&workerResp); err != nil && err != io.EOF {
		return Error(c, http.StatusBadGateway, "could not decode worker response")
	}

	if workerResp.Error != "" {
		return Error(c, http.StatusBadGateway, workerResp.Error)
	}

	if workerResp.Data == nil {
		workerResp.Data = map[string]any{"status": "queued"}
	}

	return Success(c, http.StatusOK, "enrichment job queued", workerResp.Data)
}
