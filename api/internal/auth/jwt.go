package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims defines the payload encoded for authenticated users.
type Claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	Role  string `json:"role"`
}

// JWTManager handles issuing and verifying HMAC signed tokens.
type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

// NewJWTManager constructs a manager with the given secret and token lifetime.
func NewJWTManager(secret string, ttl time.Duration) *JWTManager {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &JWTManager{secret: []byte(secret), ttl: ttl}
}

// GenerateToken creates a short-lived access token for the provided subject.
func (m *JWTManager) GenerateToken(subject, email, role string) (string, error) {
	if len(m.secret) == 0 {
		return "", errors.New("jwt secret must not be empty")
	}

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Email: email,
		Role:  role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", err
	}

	return signed, nil
}

// ParseToken verifies the token signature and payload integrity.
func (m *JWTManager) ParseToken(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}
