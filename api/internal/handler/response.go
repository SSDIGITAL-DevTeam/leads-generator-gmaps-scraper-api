package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// APIResponse describes the standard envelope returned by the API.
type APIResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// Success sends a successful response using the shared envelope format.
func Success(c echo.Context, status int, message string, data any) error {
	if status == 0 {
		status = http.StatusOK
	}
	payload := APIResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	}
	return c.JSON(status, payload)
}

// Error sends an error response using the shared envelope format.
func Error(c echo.Context, status int, message string) error {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	payload := APIResponse{
		Status:  "error",
		Message: message,
	}
	return c.JSON(status, payload)
}
