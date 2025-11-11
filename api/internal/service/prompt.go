package service

import (
	"errors"
	"regexp"
	"strings"

	"github.com/octobees/leads-generator/api/internal/dto"
)

var (
	stopwordExpr    = regexp.MustCompile(`(?i)\b(cariin|cari|tolong|minta|mohon|mau|aku|saya|butuh|yang|untuk|dong|please)\b`)
	locationPattern = regexp.MustCompile(`(?i)\b(?:di|in)\s+([a-zA-Z\s]+)`)
)

// PromptService interprets free-form search prompts.
type PromptService struct {
	DefaultCountry string
}

// PromptResult contains structured parameters derived from a prompt.
type PromptResult struct {
	TypeBusiness string
	City         string
	Country      string
	MinRating    float64
}

// NewPromptService creates a prompt parser with sensible defaults.
func NewPromptService(defaultCountry string) *PromptService {
	if strings.TrimSpace(defaultCountry) == "" {
		defaultCountry = "Indonesia"
	}
	return &PromptService{DefaultCountry: defaultCountry}
}

// Parse converts a prompt request into a structured search query.
func (s *PromptService) Parse(req dto.PromptSearchRequest) (PromptResult, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return PromptResult{}, errors.New("prompt is required")
	}

	country := strings.TrimSpace(req.Country)
	if country == "" {
		country = s.DefaultCountry
	}

	city, typeBusiness := extractCityAndType(prompt)
	if city == "" {
		city = "Jakarta"
	}
	if typeBusiness == "" {
		typeBusiness = "business"
	}

	return PromptResult{
		TypeBusiness: typeBusiness,
		City:         city,
		Country:      country,
		MinRating:    max(0, req.MinRating),
	}, nil
}

func extractCityAndType(prompt string) (string, string) {
	match := locationPattern.FindStringSubmatch(prompt)
	city := ""
	if len(match) > 1 {
		city = titleCase(match[1])
	}

	lower := strings.ToLower(prompt)
	if len(match) > 0 {
		idx := strings.Index(lower, strings.ToLower(match[0]))
		if idx >= 0 {
			prompt = strings.TrimSpace(prompt[:idx])
		}
	}

	typeBusiness := stopwordExpr.ReplaceAllString(prompt, "")
	typeBusiness = strings.TrimSpace(typeBusiness)
	if typeBusiness == "" && city != "" {
		typeBusiness = "business"
	}
	return city, typeBusiness
}

func titleCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	parts := strings.Fields(value)
	for i, p := range parts {
		lower := strings.ToLower(p)
		if len(lower) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(parts, " ")
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
