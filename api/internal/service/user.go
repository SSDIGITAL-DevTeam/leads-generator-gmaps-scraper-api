package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/octobees/leads-generator/api/internal/dto"
	"github.com/octobees/leads-generator/api/internal/repository"
)

// UserService encapsulates administrative operations for users.
type UserService struct {
	repo repository.UsersRepository
}

// NewUserService builds a new UserService instance.
func NewUserService(repo repository.UsersRepository) *UserService {
	return &UserService{repo: repo}
}

// ListUsers returns all users as DTOs.
func (s *UserService) ListUsers(ctx context.Context) ([]dto.UserResponse, error) {
	users, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.UserResponse, 0, len(users))
	for _, u := range users {
		responses = append(responses, dto.UserResponse{
			ID:    u.ID.String(),
			Email: u.Email,
			Role:  u.Role,
		})
	}
	return responses, nil
}

// CreateUser creates a new user with the supplied role.
func (s *UserService) CreateUser(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error) {
	req.Email = strings.TrimSpace(req.Email)
	req.Role = strings.TrimSpace(req.Role)

	if req.Email == "" || req.Password == "" {
		return nil, errors.New("email and password are required")
	}
	if req.Role == "" {
		req.Role = "user"
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repo.Create(ctx, req.Email, string(hashed), req.Role)
	if err != nil {
		if errors.Is(err, repository.ErrEmailDuplicate) {
			return nil, repository.ErrEmailDuplicate
		}
		return nil, err
	}

	resp := &dto.UserResponse{ID: user.ID.String(), Email: user.Email, Role: user.Role}
	return resp, nil
}

// UpdateUser mutates selected user fields.
func (s *UserService) UpdateUser(ctx context.Context, id string, req dto.UpdateUserRequest) (*dto.UserResponse, error) {
	userID, err := uuid.Parse(id)
	if err != nil {
		return nil, errors.New("invalid user id")
	}

	var emailPtr *string
	if req.Email != nil {
		trimmed := strings.TrimSpace(*req.Email)
		emailPtr = &trimmed
		if *emailPtr == "" {
			return nil, errors.New("email cannot be empty")
		}
	}

	var rolePtr *string
	if req.Role != nil {
		trimmed := strings.TrimSpace(*req.Role)
		rolePtr = &trimmed
		if *rolePtr == "" {
			return nil, errors.New("role cannot be empty")
		}
	}

	var passwordPtr *string
	if req.Password != nil {
		if strings.TrimSpace(*req.Password) == "" {
			return nil, errors.New("password cannot be empty")
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		pwd := string(hashed)
		passwordPtr = &pwd
	}

	user, err := s.repo.Update(ctx, userID, emailPtr, passwordPtr, rolePtr)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, repository.ErrUserNotFound
		}
		if errors.Is(err, repository.ErrEmailDuplicate) {
			return nil, repository.ErrEmailDuplicate
		}
		return nil, err
	}

	resp := &dto.UserResponse{ID: user.ID.String(), Email: user.Email, Role: user.Role}
	return resp, nil
}

// DeleteUser removes a user by id.
func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	userID, err := uuid.Parse(id)
	if err != nil {
		return errors.New("invalid user id")
	}
	if err := s.repo.Delete(ctx, userID); err != nil {
		return err
	}
	return nil
}
