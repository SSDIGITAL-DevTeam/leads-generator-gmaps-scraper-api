package service

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"

	"github.com/octobees/leads-generator/api/internal/auth"
	"github.com/octobees/leads-generator/api/internal/repository"
)

// AuthService coordinates credential validation and token issuance.
type AuthService struct {
	users repository.UsersRepository
	jwt   *auth.JWTManager
}

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
