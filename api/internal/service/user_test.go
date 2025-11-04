package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/entity"
	"github.com/octobees/leads-generator/api/internal/repository"
)

func TestUserService_ListUsers(t *testing.T) {
	repo := &mockUsersRepository{
		list: func(ctx context.Context) ([]entity.User, error) {
			return []entity.User{
				{ID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), Email: "admin@example.com", Role: "admin"},
				{ID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), Email: "user@example.com", Role: "user"},
			}, nil
		},
	}

	service := NewUserService(repo)
	users, err := service.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 2 || users[0].Email != "admin@example.com" || users[1].Role != "user" {
		t.Fatalf("unexpected response: %+v", users)
	}
}

func TestUserService_CreateUser(t *testing.T) {
	var capturedReq dto.CreateUserRequest
	repo := &mockUsersRepository{
		create: func(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
			capturedReq = dto.CreateUserRequest{Email: email, Role: role, Password: passwordHash}
			return &entity.User{
				ID:           uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
				Email:        email,
				PasswordHash: passwordHash,
				Role:         role,
			}, nil
		},
	}

	service := NewUserService(repo)
	req := dto.CreateUserRequest{Email: "  new@example.com ", Password: "secret", Role: "  admin "}
	resp, err := service.CreateUser(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Email != "new@example.com" || resp.Role != "admin" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if capturedReq.Role != "admin" {
		t.Fatalf("expected trimmed role, got %s", capturedReq.Role)
	}

	if _, err := service.CreateUser(context.Background(), dto.CreateUserRequest{}); err == nil {
		t.Fatalf("expected validation error for empty payload")
	}

	repo.create = func(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
		return nil, repository.ErrEmailDuplicate
	}
	if _, err := service.CreateUser(context.Background(), dto.CreateUserRequest{Email: "dup@example.com", Password: "secret"}); !errors.Is(err, repository.ErrEmailDuplicate) {
		t.Fatalf("expected email duplicate error, got %v", err)
	}
}

func TestUserService_CreateUser_DefaultRole(t *testing.T) {
	repo := &mockUsersRepository{
		create: func(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
			if role != "user" {
				t.Fatalf("expected default role user, got %s", role)
			}
			return &entity.User{ID: uuid.New(), Email: email, PasswordHash: passwordHash, Role: role}, nil
		},
	}
	service := NewUserService(repo)
	if _, err := service.CreateUser(context.Background(), dto.CreateUserRequest{Email: "user@example.com", Password: "secret"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserService_UpdateUser(t *testing.T) {
	hashed, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	repo := &mockUsersRepository{
		update: func(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error) {
			if email != nil && *email == "" {
				t.Fatalf("email should have been validated before repository call")
			}
			return &entity.User{ID: id, Email: "updated@example.com", Role: "manager", PasswordHash: string(hashed)}, nil
		},
	}

	service := NewUserService(repo)
	resp, err := service.UpdateUser(context.Background(), uuid.NewString(), dto.UpdateUserRequest{
		Email:    stringPtr(" updated@example.com "),
		Role:     stringPtr(" manager "),
		Password: stringPtr("newpass"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Email != "updated@example.com" || resp.Role != "manager" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	if _, err := service.UpdateUser(context.Background(), "bad-uuid", dto.UpdateUserRequest{}); err == nil {
		t.Fatalf("expected error for invalid uuid")
	}

	if _, err := service.UpdateUser(context.Background(), uuid.NewString(), dto.UpdateUserRequest{Email: stringPtr(" ")}); err == nil {
		t.Fatalf("expected error for empty email")
	}

	if _, err := service.UpdateUser(context.Background(), uuid.NewString(), dto.UpdateUserRequest{Role: stringPtr(" ")}); err == nil {
		t.Fatalf("expected error for empty role")
	}

	if _, err := service.UpdateUser(context.Background(), uuid.NewString(), dto.UpdateUserRequest{Password: stringPtr(" ")}); err == nil {
		t.Fatalf("expected error for empty password")
	}

	repo.update = func(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error) {
		return nil, repository.ErrUserNotFound
	}
	if _, err := service.UpdateUser(context.Background(), uuid.NewString(), dto.UpdateUserRequest{}); !errors.Is(err, repository.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}

	repo.update = func(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error) {
		return nil, repository.ErrEmailDuplicate
	}
	if _, err := service.UpdateUser(context.Background(), uuid.NewString(), dto.UpdateUserRequest{}); !errors.Is(err, repository.ErrEmailDuplicate) {
		t.Fatalf("expected ErrEmailDuplicate, got %v", err)
	}
}

func TestUserService_DeleteUser(t *testing.T) {
	repo := &mockUsersRepository{
		delete: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}
	service := NewUserService(repo)

	if err := service.DeleteUser(context.Background(), uuid.NewString()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := service.DeleteUser(context.Background(), "bad-uuid"); err == nil {
		t.Fatalf("expected invalid uuid error")
	}

	repo.delete = func(ctx context.Context, id uuid.UUID) error {
		return repository.ErrUserNotFound
	}
	if err := service.DeleteUser(context.Background(), uuid.NewString()); !errors.Is(err, repository.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}
