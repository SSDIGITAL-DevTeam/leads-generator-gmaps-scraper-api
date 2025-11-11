package service

import (
	"testing"

	"github.com/octobees/leads-generator/api/internal/dto"
)

func TestPromptService_Parse(t *testing.T) {
	service := NewPromptService("Indonesia")
	result, err := service.Parse(dto.PromptSearchRequest{Prompt: "cari 3 PT di Jakarta yang belum memiliki website", MinRating: 4})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.City != "Jakarta" {
		t.Fatalf("expected Jakarta, got %s", result.City)
	}
	if result.TypeBusiness != "PT" {
		t.Fatalf("expected PT, got %s", result.TypeBusiness)
	}
	if result.Country != "Indonesia" {
		t.Fatalf("expected default country, got %s", result.Country)
	}
	if result.Limit != 3 {
		t.Fatalf("expected limit 3, got %d", result.Limit)
	}
	if !result.RequireNoWebsite {
		t.Fatalf("expected RequireNoWebsite true")
	}
}
