package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/octobees/leads-generator/api/internal/auth"
	"github.com/octobees/leads-generator/api/internal/entity"
	"github.com/octobees/leads-generator/api/internal/repository"
)

type mockUsersRepository struct {
	findByEmail func(ctx context.Context, email string) (*entity.User, error)
	findByID    func(ctx context.Context, id uuid.UUID) (*entity.User, error)
	create      func(ctx context.Context, email, passwordHash, role string) (*entity.User, error)
	list        func(ctx context.Context) ([]entity.User, error)
	update      func(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error)
	delete      func(ctx context.Context, id uuid.UUID) error
}

func (m *mockUsersRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	if m.findByEmail != nil {
		return m.findByEmail(ctx, email)
	}
	return nil, errors.New("findByEmail not implemented")
}

func (m *mockUsersRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	if m.findByID != nil {
		return m.findByID(ctx, id)
	}
	return nil, errors.New("FindByID not implemented")
}

func (m *mockUsersRepository) Create(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
	if m.create != nil {
		return m.create(ctx, email, passwordHash, role)
	}
	return nil, errors.New("create not implemented")
}

func (m *mockUsersRepository) List(ctx context.Context) ([]entity.User, error) {
	if m.list != nil {
		return m.list(ctx)
	}
	return nil, errors.New("List not implemented")
}

func (m *mockUsersRepository) Update(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error) {
	if m.update != nil {
		return m.update(ctx, id, email, passwordHash, role)
	}
	return nil, errors.New("Update not implemented")
}

func (m *mockUsersRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if m.delete != nil {
		return m.delete(ctx, id)
	}
	return errors.New("Delete not implemented")
}

func TestAuthService_Login(t *testing.T) {
	hashed, err := bcrypt.GenerateFromPassword([]byte("super-secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("unexpected bcrypt error: %v", err)
	}

	tests := map[string]struct {
		email       string
		password    string
		repo        repository.UsersRepository
		expectError string
	}{
		"empty credentials": {
			email:       "",
			password:    "",
			repo:        &mockUsersRepository{},
			expectError: "email and password must not be empty",
		},
		"user not found": {
			email:    "john@example.com",
			password: "whatever",
			repo: &mockUsersRepository{
				findByEmail: func(ctx context.Context, email string) (*entity.User, error) {
					return nil, repository.ErrUserNotFound
				},
			},
			expectError: "invalid credentials",
		},
		"password mismatch": {
			email:    "john@example.com",
			password: "wrong",
			repo: &mockUsersRepository{
				findByEmail: func(ctx context.Context, email string) (*entity.User, error) {
					return &entity.User{
						ID:           uuid.New(),
						Email:        email,
						PasswordHash: string(hashed),
						Role:         "user",
					}, nil
				},
			},
			expectError: "invalid credentials",
		},
		"success": {
			email:    "john@example.com",
			password: "super-secret",
			repo: &mockUsersRepository{
				findByEmail: func(ctx context.Context, email string) (*entity.User, error) {
					return &entity.User{
						ID:           uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
						Email:        email,
						PasswordHash: string(hashed),
						Role:         "admin",
					}, nil
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			jwtManager := auth.NewJWTManager("test-secret", 0)
			service := NewAuthService(tt.repo, jwtManager)

			token, err := service.Login(context.Background(), tt.email, tt.password)
			if tt.expectError != "" {
				if err == nil || err.Error() != tt.expectError {
					t.Fatalf("expected error %q, got %v", tt.expectError, err)
				}
				if token != "" {
					t.Fatalf("expected empty token on error, got %q", token)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token == "" {
				t.Fatalf("expected non-empty token")
			}
		})
	}
}

func TestAuthService_Register(t *testing.T) {
	tests := map[string]struct {
		email       string
		password    string
		repo        repository.UsersRepository
		expectError error
	}{
		"empty payload": {
			expectError: errors.New("email and password must not be empty"),
			repo:        &mockUsersRepository{},
		},
		"duplicate email": {
			email:    "john@example.com",
			password: "password123",
			repo: &mockUsersRepository{
				create: func(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
					return nil, repository.ErrEmailDuplicate
				},
			},
			expectError: ErrEmailAlreadyExists,
		},
		"success": {
			email:    "jane@example.com",
			password: "password123",
			repo: &mockUsersRepository{
				create: func(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
					return &entity.User{
						ID:           uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
						Email:        email,
						PasswordHash: passwordHash,
						Role:         role,
					}, nil
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			jwtManager := auth.NewJWTManager("register-secret", 0)
			service := NewAuthService(tt.repo, jwtManager)

			token, err := service.Register(context.Background(), tt.email, tt.password)
			if tt.expectError != nil {
				if err == nil || err.Error() != tt.expectError.Error() {
					t.Fatalf("expected error %v, got %v", tt.expectError, err)
				}
				if token != "" {
					t.Fatalf("expected empty token on error, got %q", token)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token == "" {
				t.Fatalf("expected token to be returned")
			}
		})
	}
}
