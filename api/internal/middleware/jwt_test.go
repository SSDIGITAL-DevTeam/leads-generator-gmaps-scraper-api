package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/auth"
)

func TestJWTMiddleware(t *testing.T) {
	e := echo.New()
	manager := auth.NewJWTManager("secret", 0)

	token, err := manager.GenerateToken("user-1", "user@example.com", "admin")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	tests := map[string]struct {
		header     string
		expectCode int
	}{
		"missing header": {
			expectCode: http.StatusUnauthorized,
		},
		"invalid header": {
			header:     "Basic token",
			expectCode: http.StatusUnauthorized,
		},
		"invalid token": {
			header:     "Bearer invalid",
			expectCode: http.StatusUnauthorized,
		},
		"success": {
			header:     "Bearer " + token,
			expectCode: http.StatusOK,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			executed := false
			mw := JWT(manager)
			err := mw(func(c echo.Context) error {
				executed = true
				if c.Get(ContextKeyUserID) != "user-1" {
					t.Fatalf("expected user id in context")
				}
				return c.NoContent(http.StatusOK)
			})(c)

			if tt.expectCode == http.StatusOK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !executed {
					t.Fatalf("expected next handler to be executed")
				}
			} else {
				if err != nil {
					t.Fatalf("middleware returned error: %v", err)
				}
				if rec.Code != tt.expectCode {
					t.Fatalf("expected status %d, got %d", tt.expectCode, rec.Code)
				}
			}
		})
	}
}
