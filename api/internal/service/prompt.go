package service

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/octobees/leads-generator/api/internal/dto"
)

type cityAlias struct {
	match     string
	canonical string
}

var (
	stopwordExpr     = regexp.MustCompile(`(?i)\b(cariin|cari|tolong|minta|mohon|mau|aku|saya|butuh|yang|untuk|dong|please|find|search|looking|look|for|i|need|want|get|collect)\b`)
	locationPattern  = regexp.MustCompile(`(?i)\b(?:di|in|at|around|near|dekat|sekitar|sekitaran)\s+([a-z0-9\s.,-]+)`)
	numberPattern    = regexp.MustCompile(`(?i)\b(\d+)\b`)
	nowebsitePattern = regexp.MustCompile(`(?i)(belum\s+(punya|memiliki)\s+website|tanpa\s+website|without\s+(a\s+)?website|no\s+website)`)
	intentKeywords   = regexp.MustCompile(`(?i)\b(cari|search|find|scrape|look|looking|discover)\b`)
	knownCities      = []cityAlias{
		{"malioboro", "Malioboro"},
		{"jakarta", "Jakarta"},
		{"yogyakarta", "Yogyakarta"},
		{"jogja", "Yogyakarta"},
		{"yogya", "Yogyakarta"},
		{"surabaya", "Surabaya"},
		{"bandung", "Bandung"},
		{"bali", "Bali"},
		{"denpasar", "Denpasar"},
		{"semarang", "Semarang"},
		{"medan", "Medan"},
		{"malang", "Malang"},
		{"tangerang", "Tangerang"},
		{"bekasi", "Bekasi"},
		{"bogor", "Bogor"},
		{"solo", "Solo"},
		{"samarinda", "Samarinda"},
		{"balikpapan", "Balikpapan"},
		{"makassar", "Makassar"},
		{"palembang", "Palembang"},
		{"depok", "Depok"},
		{"cirebon", "Cirebon"},
	}
)

// PromptService interprets free-form search prompts.
type PromptService struct {
	DefaultCountry string
}

// PromptResult contains structured parameters derived from a prompt.
type PromptResult struct {
	TypeBusiness     string
	City             string
	Country          string
	MinRating        float64
	Limit            int
	RequireNoWebsite bool
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
	if !intentKeywords.MatchString(prompt) {
		return PromptResult{}, errors.New("prompt tidak dikenali. Gunakan kalimat seperti 'cari PT di Jakarta' untuk mencari data kontak")
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

	// limit := req.Limit
	// if limit == 0 {
	// 	limit = extractLimit(prompt)
	// }
	// if limit <= 0 {
	// 	limit = 20
	// }
	// if limit > 20 {
	// 	limit = 20
	// }
	requireNoWebsite := nowebsitePattern.MatchString(prompt)

	return PromptResult{
		TypeBusiness:     typeBusiness,
		City:             city,
		Country:          country,
		MinRating:        max(0, req.MinRating),
		// Limit:            limit,
		RequireNoWebsite: requireNoWebsite,
	}, nil
}

func extractCityAndType(prompt string) (string, string) {
	original := prompt
	match := locationPattern.FindStringSubmatch(prompt)
	city := ""
	if len(match) > 1 {
		city = deriveCityFromSegment(match[1])
	}

	lower := strings.ToLower(original)
	if len(match) > 0 {
		idx := strings.Index(lower, strings.ToLower(match[0]))
		if idx >= 0 {
			before := strings.TrimSpace(original[:idx])
			after := strings.TrimSpace(original[idx+len(match[0]):])
			switch {
			case before == "":
				prompt = after
			case after == "":
				prompt = before
			default:
				prompt = strings.TrimSpace(before + " " + after)
			}
			original = prompt
			lower = strings.ToLower(original)
		}
	}
	if city == "" {
		for _, alias := range knownCities {
			if idx := strings.Index(lower, alias.match); idx >= 0 {
				city = alias.canonical
				before := strings.TrimSpace(original[:idx])
				after := strings.TrimSpace(original[idx+len(alias.match):])
				switch {
				case before == "":
					prompt = after
				case after == "":
					prompt = before
				default:
					prompt = strings.TrimSpace(before + " " + after)
				}
				break
			}
		}
	}

	typeBusiness := stopwordExpr.ReplaceAllString(prompt, "")
	typeBusiness = stripNumbers(strings.TrimSpace(typeBusiness))
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

var keywordStops = []string{" yang", " yg", " tanpa", " without", " dengan", " dan", " that", " which", " who", " near", " around", " with", " having"}

func stripTrailingKeywords(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	cut := len(value)
	for _, kw := range keywordStops {
		if idx := strings.Index(lower, kw); idx >= 0 && idx < cut {
			cut = idx
		}
	}
	return strings.TrimSpace(value[:cut])
}

func extractLimit(prompt string) int {
	match := numberPattern.FindStringSubmatch(prompt)
	if len(match) < 2 {
		return 0
	}
	val, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return val
}

func stripNumbers(value string) string {
	cleaned := numberPattern.ReplaceAllString(value, " ")
	return strings.TrimSpace(cleaned)
}

func deriveCityFromSegment(segment string) string {
	cleaned := stripTrailingKeywords(segment)
	if cleaned == "" {
		return ""
	}
	normalized := strings.ToLower(cleaned)
	for _, alias := range knownCities {
		if strings.Contains(normalized, alias.match) {
			return alias.canonical
		}
	}
	return titleCase(cleaned)
}
