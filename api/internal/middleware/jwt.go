package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	authpkg "github.com/octobees/leads-generator/api/internal/auth"
)

// JWT validates bearer tokens and stores user metadata in the request context.
func JWT(manager *authpkg.JWTManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing authorization header"})
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid authorization header"})
			}

			claims, err := manager.ParseToken(parts[1])
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			}

			c.Set(ContextKeyUserID, claims.Subject)
			c.Set(ContextKeyUserEmail, claims.Email)
			c.Set(ContextKeyUserRole, claims.Role)

			return next(c)
		}
	}
}
