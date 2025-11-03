package service

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/octobees/leads-generator/api/internal/auth"
	"github.com/octobees/leads-generator/api/internal/repository"
)

// AuthService coordinates credential validation and token issuance.
type AuthService struct {
	users repository.UsersRepository
	jwt   *auth.JWTManager
}

// ErrEmailAlreadyExists indicates a duplicate registration attempt.
var ErrEmailAlreadyExists = errors.New("email already exists")

// NewAuthService constructs a new AuthService.
func NewAuthService(users repository.UsersRepository, jwtManager *auth.JWTManager) *AuthService {
	return &AuthService{users: users, jwt: jwtManager}
}

// Login validates credentials and returns a JWT.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	if email == "" || password == "" {
		return "", errors.New("email and password must not be empty")
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return "", errors.New("invalid credentials")
		}
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}

	token, err := s.jwt.GenerateToken(user.ID.String(), user.Email, user.Role)
	if err != nil {
		return "", err
	}

	return token, nil
}

// Register creates a new user with the default role and returns an access token.
func (s *AuthService) Register(ctx context.Context, email, password string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return "", errors.New("email and password must not be empty")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	user, err := s.users.Create(ctx, email, string(hashed), "user")
	if err != nil {
		if errors.Is(err, repository.ErrEmailDuplicate) {
			return "", ErrEmailAlreadyExists
		}
		return "", err
	}

	token, err := s.jwt.GenerateToken(user.ID.String(), user.Email, user.Role)
	if err != nil {
		return "", err
	}

	return token, nil
}
