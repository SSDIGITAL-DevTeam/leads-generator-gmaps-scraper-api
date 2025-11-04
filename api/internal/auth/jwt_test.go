package auth

import (
	"testing"
	"time"
)

func TestJWTManager_GenerateAndParse(t *testing.T) {
	manager := NewJWTManager("secret", time.Hour)
	token, err := manager.GenerateToken("user-1", "user@example.com", "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	claims, err := manager.ParseToken(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Subject != "user-1" || claims.Email != "user@example.com" || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	if _, err := manager.ParseToken(token + "tampered"); err == nil {
		t.Fatalf("expected parse error for tampered token")
	}
}

func TestJWTManager_EmptySecret(t *testing.T) {
	manager := NewJWTManager("", time.Hour)
	if _, err := manager.GenerateToken("user", "user@example.com", "user"); err == nil {
		t.Fatalf("expected error when secret is empty")
	}
}
