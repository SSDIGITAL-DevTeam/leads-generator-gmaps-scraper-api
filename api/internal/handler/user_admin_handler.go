package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/repository"
	"github.com/octobees/leads-generator/api/internal/service"
)

// UserAdminHandler exposes administrative user management endpoints.
type UserAdminHandler struct {
	users *service.UserService
}

// NewUserAdminHandler constructs a handler instance.
func NewUserAdminHandler(users *service.UserService) *UserAdminHandler {
	return &UserAdminHandler{users: users}
}

// List returns all users.
func (h *UserAdminHandler) List(c echo.Context) error {
	records, err := h.users.ListUsers(c.Request().Context())
	if err != nil {
		return Error(c, http.StatusInternalServerError, "failed to list users")
	}
	return Success(c, http.StatusOK, "users retrieved", records)
}

// Create provisions a new user.
func (h *UserAdminHandler) Create(c echo.Context) error {
	var req dto.CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, http.StatusBadRequest, "invalid payload")
	}

	user, err := h.users.CreateUser(c.Request().Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrEmailDuplicate):
			return Error(c, http.StatusConflict, "email already exists")
		default:
			return Error(c, http.StatusBadRequest, err.Error())
		}
	}

	return Success(c, http.StatusCreated, "user created", user)
}

// Update modifies an existing user.
func (h *UserAdminHandler) Update(c echo.Context) error {
	id := c.Param("id")
	var req dto.UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, http.StatusBadRequest, "invalid payload")
	}

	user, err := h.users.UpdateUser(c.Request().Context(), id, req)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrUserNotFound):
			return Error(c, http.StatusNotFound, "user not found")
		case errors.Is(err, repository.ErrEmailDuplicate):
			return Error(c, http.StatusConflict, "email already exists")
		default:
			return Error(c, http.StatusBadRequest, err.Error())
		}
	}

	return Success(c, http.StatusOK, "user updated", user)
}

// Delete removes a user.
func (h *UserAdminHandler) Delete(c echo.Context) error {
	id := c.Param("id")
	if err := h.users.DeleteUser(c.Request().Context(), id); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return Error(c, http.StatusNotFound, "user not found")
		}
		if err.Error() == "invalid user id" {
			return Error(c, http.StatusBadRequest, err.Error())
		}
		return Error(c, http.StatusInternalServerError, "failed to delete user")
	}

	return Success(c, http.StatusOK, "user deleted", nil)
}
