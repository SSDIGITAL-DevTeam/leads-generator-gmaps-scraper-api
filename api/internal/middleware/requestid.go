package middleware

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// RequestID injects an identifier for traceability if the caller did not provide one.
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			rid := c.Request().Header.Get("X-Request-ID")
			if rid == "" {
				rid = uuid.NewString()
			}

			c.Set(ContextKeyRequestID, rid)
			c.Response().Header().Set("X-Request-ID", rid)

			return next(c)
		}
	}
}

// RequestIDFromContext extracts the request identifier if available.
func RequestIDFromContext(c echo.Context) string {
	if val, ok := c.Get(ContextKeyRequestID).(string); ok {
		return val
	}
	return ""
}
