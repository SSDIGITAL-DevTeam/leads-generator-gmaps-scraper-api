package middleware

import (
	"log"
	"time"

	"github.com/labstack/echo/v4"
)

// Logging writes a concise structured line for each HTTP request.
func Logging() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			latency := time.Since(start)

			if err != nil {
				c.Error(err)
			}

			rid, _ := c.Get(ContextKeyRequestID).(string)
			log.Printf("request_id=%s method=%s path=%s status=%d latency=%s", rid, c.Request().Method, c.Request().URL.Path, c.Response().Status, latency)

			return err
		}
	}
}
