package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// RequireRole enforces that the authenticated request carries the expected role.
func RequireRole(role string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			value, ok := c.Get(ContextKeyUserRole).(string)
			if !ok || value == "" {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "missing role"})
			}
			if value != role {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
			}
			return next(c)
		}
	}
}
