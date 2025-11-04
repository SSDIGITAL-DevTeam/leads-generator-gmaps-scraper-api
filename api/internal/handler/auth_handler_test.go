package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/octobees/leads-generator/api/internal/auth"
	"github.com/octobees/leads-generator/api/internal/entity"
	"github.com/octobees/leads-generator/api/internal/repository"
	"github.com/octobees/leads-generator/api/internal/service"
)

type stubUsersRepo struct {
	findByEmail func(ctx context.Context, email string) (*entity.User, error)
	create      func(ctx context.Context, email, passwordHash, role string) (*entity.User, error)
}

func (s *stubUsersRepo) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	if s.findByEmail != nil {
		return s.findByEmail(ctx, email)
	}
	return nil, errors.New("not implemented")
}

func (s *stubUsersRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUsersRepo) Create(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
	if s.create != nil {
		return s.create(ctx, email, passwordHash, role)
	}
	return nil, errors.New("not implemented")
}

func (s *stubUsersRepo) List(ctx context.Context) ([]entity.User, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUsersRepo) Update(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUsersRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return errors.New("not implemented")
}

func newAuthHandler(t *testing.T, repo repository.UsersRepository) *AuthHandler {
	t.Helper()
	jwtManager := auth.NewJWTManager("test-secret", 0)
	service := service.NewAuthService(repo, jwtManager)
	return NewAuthHandler(service)
}

func TestAuthHandler_Register(t *testing.T) {
	e := echo.New()

	t.Run("invalid payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString("{"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newAuthHandler(t, &stubUsersRepo{})
		if err := handler.Register(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		payload := map[string]string{"email": " ", "password": ""}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newAuthHandler(t, &stubUsersRepo{})
		_ = handler.Register(c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		payload := map[string]string{"email": "user@example.com", "password": "secret"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newAuthHandler(t, &stubUsersRepo{
			create: func(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
				return nil, repository.ErrEmailDuplicate
			},
		})

		_ = handler.Register(c)
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		payload := map[string]string{"email": "user@example.com", "password": "secret"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newAuthHandler(t, &stubUsersRepo{
			create: func(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
				return &entity.User{ID: uuid.New(), Email: email, PasswordHash: passwordHash, Role: role}, nil
			},
		})

		_ = handler.Register(c)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rec.Code)
		}
	})
}

func TestAuthHandler_Login(t *testing.T) {
	e := echo.New()
	hashed, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)

	t.Run("invalid payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString("{"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newAuthHandler(t, &stubUsersRepo{})
		_ = handler.Login(c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("invalid credentials", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"email": "user@example.com", "password": "wrong"})
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newAuthHandler(t, &stubUsersRepo{
			findByEmail: func(ctx context.Context, email string) (*entity.User, error) {
				return &entity.User{ID: uuid.New(), Email: email, PasswordHash: string(hashed), Role: "user"}, nil
			},
		})

		_ = handler.Login(c)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("unexpected error", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"email": "user@example.com", "password": "secret"})
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newAuthHandler(t, &stubUsersRepo{
			findByEmail: func(ctx context.Context, email string) (*entity.User, error) {
				return nil, errors.New("db down")
			},
		})

		_ = handler.Login(c)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"email": "user@example.com", "password": "secret"})
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := newAuthHandler(t, &stubUsersRepo{
			findByEmail: func(ctx context.Context, email string) (*entity.User, error) {
				return &entity.User{ID: uuid.New(), Email: email, PasswordHash: string(hashed), Role: "user"}, nil
			},
		})

		_ = handler.Login(c)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})
}
