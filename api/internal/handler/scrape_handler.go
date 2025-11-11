package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/dto"
	middleware "github.com/octobees/leads-generator/api/internal/middleware"
)

// ScrapeHandler posts scrape requests to the worker service.
type ScrapeHandler struct {
	worker WorkerPoster
}

// NewScrapeHandler constructs a scrape handler backed by an HTTP client.
// If `client == nil`, it automatically creates an ID-token client for Cloud Run → Cloud Run calls.
func NewScrapeHandler(client *http.Client, workerBaseURL string) *ScrapeHandler {
	return &ScrapeHandler{worker: NewWorkerClient(client, workerBaseURL)}
}

// NewScrapeHandlerWithWorker allows injecting a custom worker client (useful for tests).
func NewScrapeHandlerWithWorker(worker WorkerPoster) *ScrapeHandler {
	return &ScrapeHandler{worker: worker}
}

// Enqueue handles POST /scrape requests and forwards them to the worker.
func (h *ScrapeHandler) Enqueue(c echo.Context) error {
	var req dto.ScrapeRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, http.StatusBadRequest, "invalid payload")
	}

	// Normalize fields
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

	payload := map[string]any{
		"type_business": req.TypeBusiness,
		"city":          req.City,
		"country":       req.Country,
	}
	if req.MinRating > 0 {
		payload["min_rating"] = req.MinRating
	}

	ctx := c.Request().Context()
	data, err := h.worker.PostJSON(ctx, "/scrape", payload, middleware.RequestIDFromContext(c))
	if err != nil {
		return Error(c, http.StatusBadGateway, err.Error())
	}
	if data == nil {
		data = map[string]any{"status": "queued"}
	}
	return Success(c, http.StatusOK, "scrape job queued", data)
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
