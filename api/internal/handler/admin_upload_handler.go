package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/service"
)

// AdminUploadHandler handles CSV ingestion for administrators.
type AdminUploadHandler struct {
	companiesService *service.CompaniesService
}

// NewAdminUploadHandler wires a handler backed by the companies service.
func NewAdminUploadHandler(companiesService *service.CompaniesService) *AdminUploadHandler {
	return &AdminUploadHandler{companiesService: companiesService}
}

// UploadCSV handles POST /admin/upload-csv requests.
func (h *AdminUploadHandler) UploadCSV(c echo.Context) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return Error(c, http.StatusBadRequest, "missing csv file")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return Error(c, http.StatusBadRequest, "unable to open file")
	}
	defer file.Close()

	summary, err := h.companiesService.ImportCompaniesCSV(c.Request().Context(), file)
	if err != nil {
		var validationErr service.CSVValidationError
		if errors.As(err, &validationErr) {
			return Error(c, http.StatusBadRequest, validationErr.Error())
		}
		return Error(c, http.StatusInternalServerError, "failed to process csv")
	}

	return Success(c, http.StatusOK, "companies CSV processed", summary)
}
