package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/service"
)

// AuthHandler exposes authentication endpoints.
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register handles POST /auth/register requests.
func (h *AuthHandler) Register(c echo.Context) error {
	var req dto.RegisterRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, http.StatusBadRequest, "invalid payload")
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		return Error(c, http.StatusBadRequest, "email and password are required")
	}

	token, err := h.authService.Register(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailAlreadyExists):
			return Error(c, http.StatusConflict, "email already exists")
		default:
			return Error(c, http.StatusInternalServerError, "unable to register user")
		}
	}

	return Success(c, http.StatusCreated, "registration successful", dto.LoginResponse{AccessToken: token})
}

// Login handles POST /auth/login requests.
func (h *AuthHandler) Login(c echo.Context) error {
	var req dto.LoginRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, http.StatusBadRequest, "invalid payload")
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		return Error(c, http.StatusBadRequest, "email and password are required")
	}

	token, err := h.authService.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "invalid credentials") {
			return Error(c, http.StatusUnauthorized, "invalid credentials")
		}
		return Error(c, http.StatusInternalServerError, "unable to authenticate")
	}

	return Success(c, http.StatusOK, "login successful", dto.LoginResponse{AccessToken: token})
}
