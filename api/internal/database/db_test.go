package database

import (
	"context"
	"testing"
)

func TestConnect_Validation(t *testing.T) {
	if _, err := Connect(context.Background(), ""); err == nil {
		t.Fatalf("expected error for empty dsn")
	}

	if _, err := Connect(context.Background(), "invalid-dsn"); err == nil {
		t.Fatalf("expected error for invalid dsn")
	}
}
