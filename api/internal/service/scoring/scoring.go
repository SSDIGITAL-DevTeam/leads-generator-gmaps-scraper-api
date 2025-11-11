package scoring

import (
	"net/url"
	"strings"
	"unicode"
)

const (
	categoryContact  = "contact_completeness"
	categoryWebsite  = "website_quality"
	categorySocial   = "social_presence"
	categoryBusiness = "business_profile"
)

var freeHostingDomains = []string{
	"wordpress.com",
	"blogspot.com",
	"wixsite.com",
	"weebly.com",
	"squarespace.com",
	"medium.com",
	"substack.com",
	"godaddysites.com",
	"notion.site",
	"googlepages.com",
}

// LeadFeatures captures the enrichment signals used for scoring.
type LeadFeatures struct {
	Emails         []string
	Phones         []string
	Socials        map[string]string
	HasHTTPS       bool
	HasContactPage bool
	HasAboutPage   bool
	HasContactForm bool
	Address        string
	Website        string
}

// ScoreResult reports the aggregate score and the per-category breakdown.
type ScoreResult struct {
	Total     int
	Breakdown map[string]int
}

// ComputeScore evaluates the provided features and returns the score breakdown.
func ComputeScore(input LeadFeatures) ScoreResult {
	breakdown := map[string]int{
		categoryContact:  scoreContactCompleteness(input),
		categoryWebsite:  scoreWebsiteQuality(input),
		categorySocial:   scoreSocialPresence(input),
		categoryBusiness: scoreBusinessProfile(input),
	}

	total := 0
	for _, value := range breakdown {
		total += value
	}

	return ScoreResult{
		Total:     total,
		Breakdown: breakdown,
	}
}

func scoreContactCompleteness(input LeadFeatures) int {
	score := 0
	if hasValue(input.Emails) {
		score += 10
	}
	if hasValue(input.Phones) {
		score += 10
	}
	score += min(countSocialLinks(input.Socials), 10)
	if score > 30 {
		return 30
	}
	return score
}

func scoreWebsiteQuality(input LeadFeatures) int {
	score := 0
	if hasHTTPS(input) {
		score += 10
	}
	if input.HasContactPage {
		score += 10
	}
	if input.HasAboutPage {
		score += 5
	}
	if input.HasContactForm {
		score += 5
	}
	if score > 30 {
		return 30
	}
	return score
}

func scoreSocialPresence(input LeadFeatures) int {
	if len(input.Socials) == 0 {
		return 0
	}

	score := 0
	normalized := normalizeSocialKeys(input.Socials)
	if normalized["linkedin"] != "" {
		score += 5
	}
	if normalized["instagram"] != "" {
		score += 5
	}
	if normalized["facebook"] != "" {
		score += 5
	}
	if normalized["youtube"] != "" || normalized["tiktok"] != "" {
		score += 5
	}
	if score > 20 {
		return 20
	}
	return score
}

func scoreBusinessProfile(input LeadFeatures) int {
	score := 0
	if hasCompleteAddress(input.Address) {
		score += 10
	}
	if highQualityDomain(input.Website) {
		score += 10
	}
	if score > 20 {
		return 20
	}
	return score
}

func hasValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func countSocialLinks(socials map[string]string) int {
	if len(socials) == 0 {
		return 0
	}
	count := 0
	for _, payload := range socials {
		for _, token := range splitSocialValue(payload) {
			if token != "" {
				count++
			}
		}
	}
	return count
}

func splitSocialValue(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '|':
			return true
		default:
			return r == ' ' || r == '\n' || r == '\t' || r == '\r'
		}
	})
}

func normalizeSocialKeys(socials map[string]string) map[string]string {
	if len(socials) == 0 {
		return map[string]string{}
	}
	result := make(map[string]string, len(socials))
	for key, value := range socials {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		if normalizedKey == "" {
			continue
		}
		result[normalizedKey] = strings.TrimSpace(value)
	}
	return result
}

func hasHTTPS(input LeadFeatures) bool {
	if input.HasHTTPS {
		return true
	}
	site := strings.ToLower(strings.TrimSpace(input.Website))
	return strings.HasPrefix(site, "https://")
}

func hasCompleteAddress(raw string) bool {
	addr := strings.TrimSpace(raw)
	if len(addr) < 10 {
		return false
	}
	var hasLetter, hasDigit bool
	separatorCount := 0
	for _, r := range addr {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		case r == ',':
			separatorCount++
		}
	}
	return hasLetter && hasDigit && separatorCount >= 1
}

func highQualityDomain(raw string) bool {
	domain := extractDomain(raw)
	if domain == "" {
		return false
	}
	for _, bad := range freeHostingDomains {
		if domain == bad || strings.HasSuffix(domain, "."+bad) {
			return false
		}
	}
	return strings.Count(domain, ".") >= 1
}

func extractDomain(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lowered := strings.ToLower(raw)
	if !strings.Contains(lowered, "://") {
		lowered = "https://" + lowered
	}
	parsed, err := url.Parse(lowered)
	if err != nil {
		return ""
	}
	host := strings.TrimSpace(strings.ToLower(parsed.Host))
	host = strings.TrimPrefix(host, "www.")
	return host
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
