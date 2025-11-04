package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/dto"
	middleware "github.com/octobees/leads-generator/api/internal/middleware"
)

// ScrapeHandler posts scrape requests to the worker service.
type ScrapeHandler struct {
	client        *http.Client
	workerBaseURL string
}

// NewScrapeHandler constructs a scrape handler backed by the provided HTTP client.
func NewScrapeHandler(client *http.Client, workerBaseURL string) *ScrapeHandler {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &ScrapeHandler{client: client, workerBaseURL: workerBaseURL}
}

// Enqueue handles POST /scrape requests and forwards them to the worker.
func (h *ScrapeHandler) Enqueue(c echo.Context) error {
	var req dto.ScrapeRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, http.StatusBadRequest, "invalid payload")
	}

	req.TypeBusiness = strings.TrimSpace(req.TypeBusiness)
	req.City = strings.TrimSpace(req.City)
	req.Country = strings.TrimSpace(req.Country)
	if req.MinRating < 0 {
		req.MinRating = 0
	}

	if req.TypeBusiness == "" {
		return Error(c, http.StatusBadRequest, "type_business is required")
	}
	if req.City == "" || req.Country == "" {
		// tolerate legacy payloads containing only location by attempting a naive split
		if req.Location != "" {
			parts := strings.Split(req.Location, ",")
			if len(parts) >= 2 {
				req.City = strings.TrimSpace(parts[0])
				req.Country = strings.TrimSpace(parts[1])
			}
		}
	}
	if req.City == "" || req.Country == "" {
		return Error(c, http.StatusBadRequest, "city and country are required")
	}

	if h.workerBaseURL == "" {
		return Error(c, http.StatusServiceUnavailable, "worker endpoint is not configured")
	}

	payload := map[string]any{
		"type_business": req.TypeBusiness,
		"city":          req.City,
		"country":       req.Country,
	}
	if req.MinRating > 0 {
		payload["min_rating"] = req.MinRating
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "failed to marshal request")
	}

	workerURL := strings.TrimRight(h.workerBaseURL, "/") + "/scrape"
	ctx := c.Request().Context()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, workerURL, bytes.NewReader(body))
	if err != nil {
		return Error(c, http.StatusInternalServerError, "failed to create worker request")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if rid := middleware.RequestIDFromContext(c); rid != "" {
		httpReq.Header.Set("X-Request-ID", rid)
	}

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return Error(c, http.StatusBadGateway, fmt.Sprintf("worker request failed: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		workerErr := extractWorkerError(resp.Body)
		return Error(c, http.StatusBadGateway, workerErr)
	}

	var workerResp struct {
		Data  map[string]any `json:"data"`
		Error string         `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&workerResp); err != nil && !errors.Is(err, io.EOF) {
		return Error(c, http.StatusBadGateway, "could not decode worker response")
	}

	if workerResp.Error != "" {
		return Error(c, http.StatusBadGateway, workerResp.Error)
	}

	if workerResp.Data == nil {
		workerResp.Data = map[string]any{"status": "queued"}
	}

	return Success(c, http.StatusOK, "scrape job queued", workerResp.Data)
}

func extractWorkerError(body io.Reader) string {
	data, err := io.ReadAll(body)
	if err != nil {
		return "worker returned an error"
	}
	if len(data) == 0 {
		return "worker returned an error"
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &payload); err == nil && payload.Error != "" {
		return payload.Error
	}
	return string(data)
}
