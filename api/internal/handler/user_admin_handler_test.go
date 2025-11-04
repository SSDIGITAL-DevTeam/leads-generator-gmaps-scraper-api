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

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/entity"
	"github.com/octobees/leads-generator/api/internal/repository"
	"github.com/octobees/leads-generator/api/internal/service"
)

type usersRepoForHandler struct {
	list   func(ctx context.Context) ([]entity.User, error)
	create func(ctx context.Context, email, passwordHash, role string) (*entity.User, error)
	update func(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error)
	delete func(ctx context.Context, id uuid.UUID) error
}

func (u *usersRepoForHandler) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	return nil, errors.New("not implemented")
}

func (u *usersRepoForHandler) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	return nil, errors.New("not implemented")
}

func (u *usersRepoForHandler) Create(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
	if u.create != nil {
		return u.create(ctx, email, passwordHash, role)
	}
	return nil, errors.New("not implemented")
}

func (u *usersRepoForHandler) List(ctx context.Context) ([]entity.User, error) {
	if u.list != nil {
		return u.list(ctx)
	}
	return nil, errors.New("not implemented")
}

func (u *usersRepoForHandler) Update(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error) {
	if u.update != nil {
		return u.update(ctx, id, email, passwordHash, role)
	}
	return nil, errors.New("not implemented")
}

func (u *usersRepoForHandler) Delete(ctx context.Context, id uuid.UUID) error {
	if u.delete != nil {
		return u.delete(ctx, id)
	}
	return errors.New("not implemented")
}

func newUserAdminHandler(repo repository.UsersRepository) *UserAdminHandler {
	service := service.NewUserService(repo)
	return NewUserAdminHandler(service)
}

func TestUserAdminHandler_List(t *testing.T) {
	e := echo.New()
	repo := &usersRepoForHandler{
		list: func(ctx context.Context) ([]entity.User, error) {
			return []entity.User{{ID: uuid.New(), Email: "admin@example.com", Role: "admin"}}, nil
		},
	}
	handler := newUserAdminHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.List(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	repo.list = func(ctx context.Context) ([]entity.User, error) {
		return nil, errors.New("boom")
	}
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	_ = handler.List(c)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestUserAdminHandler_Create(t *testing.T) {
	e := echo.New()
	repo := &usersRepoForHandler{
		create: func(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
			return &entity.User{ID: uuid.New(), Email: email, Role: role, PasswordHash: passwordHash}, nil
		},
	}
	handler := newUserAdminHandler(repo)

	t.Run("invalid payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/users", bytes.NewBufferString("{"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_ = handler.Create(c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		repo.create = func(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
			return nil, repository.ErrEmailDuplicate
		}
		body, _ := json.Marshal(dto.CreateUserRequest{Email: "user@example.com", Password: "secret"})
		req := httptest.NewRequest(http.MethodPost, "/admin/users", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_ = handler.Create(c)
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
	})

	t.Run("other error", func(t *testing.T) {
		repo.create = func(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
			return nil, errors.New("invalid payload")
		}
		body, _ := json.Marshal(dto.CreateUserRequest{Email: "user@example.com", Password: "secret"})
		req := httptest.NewRequest(http.MethodPost, "/admin/users", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_ = handler.Create(c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		repo.create = func(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
			return &entity.User{ID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), Email: email, Role: role}, nil
		}
		body, _ := json.Marshal(dto.CreateUserRequest{Email: "user@example.com", Password: "secret"})
		req := httptest.NewRequest(http.MethodPost, "/admin/users", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_ = handler.Create(c)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rec.Code)
		}
	})
}

func TestUserAdminHandler_Update(t *testing.T) {
	e := echo.New()
	repo := &usersRepoForHandler{
		update: func(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error) {
			return &entity.User{ID: id, Email: "new@example.com", Role: "admin"}, nil
		},
	}
	handler := newUserAdminHandler(repo)

	t.Run("invalid payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/admin/users/1", bytes.NewBufferString("{"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(uuid.NewString())

		_ = handler.Update(c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo.update = func(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error) {
			return nil, repository.ErrUserNotFound
		}
		body, _ := json.Marshal(dto.UpdateUserRequest{})
		req := httptest.NewRequest(http.MethodPatch, "/admin/users/"+uuid.NewString(), bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(uuid.NewString())

		_ = handler.Update(c)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		repo.update = func(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error) {
			return nil, repository.ErrEmailDuplicate
		}
		body, _ := json.Marshal(dto.UpdateUserRequest{})
		req := httptest.NewRequest(http.MethodPatch, "/admin/users/"+uuid.NewString(), bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(uuid.NewString())

		_ = handler.Update(c)
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
	})

	t.Run("other error", func(t *testing.T) {
		repo.update = func(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error) {
			return nil, errors.New("invalid role")
		}
		body, _ := json.Marshal(dto.UpdateUserRequest{})
		req := httptest.NewRequest(http.MethodPatch, "/admin/users/"+uuid.NewString(), bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(uuid.NewString())

		_ = handler.Update(c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		repo.update = func(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error) {
			return &entity.User{ID: id, Email: "updated@example.com", Role: "manager"}, nil
		}
		body, _ := json.Marshal(dto.UpdateUserRequest{})
		req := httptest.NewRequest(http.MethodPatch, "/admin/users/"+uuid.NewString(), bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(uuid.NewString())

		_ = handler.Update(c)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})
}

func TestUserAdminHandler_Delete(t *testing.T) {
	e := echo.New()
	repo := &usersRepoForHandler{
		delete: func(ctx context.Context, id uuid.UUID) error { return nil },
	}
	handler := newUserAdminHandler(repo)

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+uuid.NewString(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(uuid.NewString())

		_ = handler.Delete(c)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/users/invalid", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("invalid")

		_ = handler.Delete(c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo.delete = func(ctx context.Context, id uuid.UUID) error {
			return repository.ErrUserNotFound
		}
		req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+uuid.NewString(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(uuid.NewString())

		_ = handler.Delete(c)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("unexpected error", func(t *testing.T) {
		repo.delete = func(ctx context.Context, id uuid.UUID) error {
			return errors.New("db down")
		}
		req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+uuid.NewString(), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(uuid.NewString())

		_ = handler.Delete(c)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}
