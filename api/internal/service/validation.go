package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/nyaruka/phonenumbers"
	"golang.org/x/net/idna"
)

var (
	emailPattern = regexp.MustCompile(`^[a-z0-9._%+\-']+@[a-z0-9.-]+\.[a-z]{2,}$`)
	idnaProfile  = idna.Lookup
)

const (
	trackingPrefix     = "utm_"
	defaultPhoneRegion = "ID"
	defaultHTTPTimeout = 5 * time.Second
)

var allowedSocialDomains = map[string]string{
	"linkedin.com":  "linkedin",
	"facebook.com":  "facebook",
	"instagram.com": "instagram",
	"youtube.com":   "youtube",
	"youtu.be":      "youtube",
	"tiktok.com":    "tiktok",
}

// CleanedData represents validated and normalized contact information.
type CleanedData struct {
	CompanyID      string      `json:"company_id"`
	Emails         []string    `json:"emails"`
	Phones         []string    `json:"phones"`
	Socials        SocialLinks `json:"socials"`
	Address        string      `json:"address"`
	ContactFormURL string      `json:"contact_form_url"`
}

// SocialLinks stores the canonical URL for each supported network.
type SocialLinks struct {
	LinkedIn  string `json:"linkedin,omitempty"`
	Facebook  string `json:"facebook,omitempty"`
	Instagram string `json:"instagram,omitempty"`
	Youtube   string `json:"youtube,omitempty"`
	Tiktok    string `json:"tiktok,omitempty"`
}

// RawEnrichedData is the unvalidated payload sent by the enrichment worker.
type RawEnrichedData struct {
	CompanyID       string
	Emails          []string
	PrimaryPhone    string
	SecondaryPhones []string
	SocialLinks     map[string][]string
	Addresses       []string
	ContactFormURL  string
}

// DNSResolver abstracts DNS lookups to simplify testing.
type DNSResolver interface {
	LookupMX(ctx context.Context, domain string) ([]*net.MX, error)
}

// HTTPClient abstracts HTTP requests for validation purposes.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// DataProcessor encapsulates the data cleaning and validation rules.
type DataProcessor struct {
	DefaultRegion string
	dnsResolver   DNSResolver
	httpClient    HTTPClient
}

// DataProcessorOption configures optional dependencies.
type DataProcessorOption func(*DataProcessor)

// WithDNSResolver overrides the default DNS resolver.
func WithDNSResolver(resolver DNSResolver) DataProcessorOption {
	return func(p *DataProcessor) {
		p.dnsResolver = resolver
	}
}

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(client HTTPClient) DataProcessorOption {
	return func(p *DataProcessor) {
		if client != nil {
			p.httpClient = client
		}
	}
}

// NewDataProcessor builds a processor with sensible defaults.
func NewDataProcessor(defaultRegion string, opts ...DataProcessorOption) *DataProcessor {
	region := strings.ToUpper(strings.TrimSpace(defaultRegion))
	if region == "" {
		region = defaultPhoneRegion
	}
	p := &DataProcessor{
		DefaultRegion: region,
		dnsResolver:   systemDNSResolver{},
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.DefaultRegion == "" {
		p.DefaultRegion = defaultPhoneRegion
	}
	return p
}

// Process executes all cleaning and validation rules and returns structured data.
func (p *DataProcessor) Process(ctx context.Context, input RawEnrichedData) (CleanedData, error) {
	companyID := strings.TrimSpace(input.CompanyID)
	if companyID == "" {
		return CleanedData{}, errors.New("company_id is required")
	}

	emails := p.cleanEmails(ctx, input.Emails)
	phones := p.normalizePhones(input.PrimaryPhone, input.SecondaryPhones)
	socials := p.validateSocials(ctx, input.SocialLinks)
	address := selectBestAddress(input.Addresses)
	contactForm := p.sanitizeContactForm(input.ContactFormURL)

	return CleanedData{
		CompanyID:      companyID,
		Emails:         emails,
		Phones:         phones,
		Socials:        socials,
		Address:        address,
		ContactFormURL: contactForm,
	}, nil
}

func (p *DataProcessor) cleanEmails(ctx context.Context, emails []string) []string {
	if len(emails) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(emails))
	domainCache := make(map[string]bool)
	valid := make([]string, 0, len(emails))

	for _, raw := range emails {
		email := strings.ToLower(strings.TrimSpace(raw))
		if email == "" || !emailPattern.MatchString(email) {
			continue
		}
		parts := strings.SplitN(email, "@", 2)
		domain := parts[1]
		if !isDomainValid(domain) {
			continue
		}
		asciiDomain, err := idnaProfile.ToASCII(domain)
		if err != nil || asciiDomain == "" {
			continue
		}
		if ok, cached := domainCache[asciiDomain]; cached {
			if !ok {
				continue
			}
		} else {
			hasMX := p.hasMXRecord(ctx, asciiDomain)
			domainCache[asciiDomain] = hasMX
			if !hasMX {
				continue
			}
		}
		if _, dup := seen[email]; dup {
			continue
		}
		seen[email] = struct{}{}
		valid = append(valid, email)
	}
	if len(valid) == 0 {
		return nil
	}
	return valid
}

func (p *DataProcessor) normalizePhones(primary string, secondary []string) []string {
	candidates := make([]string, 0, 1+len(secondary))
	if phone := strings.TrimSpace(primary); phone != "" {
		candidates = append(candidates, phone)
	}
	candidates = append(candidates, secondary...)

	seen := make(map[string]struct{}, len(candidates))
	valid := make([]string, 0, len(candidates))

	for _, raw := range candidates {
		normalized := normalizePhone(raw, p.DefaultRegion)
		if normalized == "" {
			continue
		}
		if _, dup := seen[normalized]; dup {
			continue
		}
		seen[normalized] = struct{}{}
		valid = append(valid, normalized)
	}
	if len(valid) == 0 {
		return nil
	}
	return valid
}

func (p *DataProcessor) validateSocials(ctx context.Context, socials map[string][]string) SocialLinks {
	if len(socials) == 0 {
		return SocialLinks{}
	}
	result := SocialLinks{}
	used := make(map[string]struct{})

	for key, candidates := range socials {
		platform := canonicalSocialKey(key)
		if platform == "" || len(candidates) == 0 {
			continue
		}
		if _, exists := used[platform]; exists {
			continue
		}
		for _, raw := range candidates {
			sanitized, ok := p.cleanSocialLink(ctx, platform, raw)
			if !ok {
				continue
			}
			result.set(platform, sanitized)
			used[platform] = struct{}{}
			break
		}
	}
	return result
}

func (p *DataProcessor) sanitizeContactForm(raw string) string {
	u, err := sanitizeURL(raw)
	if err != nil {
		return ""
	}
	stripTracking(u)
	return u.String()
}

func (p *DataProcessor) cleanSocialLink(ctx context.Context, platform, raw string) (string, bool) {
	u, err := sanitizeURL(raw)
	if err != nil {
		return "", false
	}
	hostPlatform, ok := hostMatchesAllowed(u.Hostname())
	if !ok || hostPlatform != platform {
		return "", false
	}
	stripTracking(u)
	if !p.urlResolves(ctx, u.String()) {
		return "", false
	}
	return u.String(), true
}

func (p *DataProcessor) hasMXRecord(ctx context.Context, domain string) bool {
	if p.dnsResolver == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	records, err := p.dnsResolver.LookupMX(ctx, domain)
	return err == nil && len(records) > 0
}

func (p *DataProcessor) urlResolves(ctx context.Context, target string) bool {
	if p.httpClient == nil {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, target, nil)
	if err != nil {
		return false
	}
	resp, err := p.httpClient.Do(req)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return true
		}
		if resp.StatusCode != http.StatusMethodNotAllowed {
			return false
		}
	}

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return false
	}
	resp, err = p.httpClient.Do(getReq)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (links *SocialLinks) set(platform, value string) {
	switch platform {
	case "linkedin":
		links.LinkedIn = value
	case "facebook":
		links.Facebook = value
	case "instagram":
		links.Instagram = value
	case "youtube":
		links.Youtube = value
	case "tiktok":
		links.Tiktok = value
	}
}

func canonicalSocialKey(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "linkedin", "linkedin_url":
		return "linkedin"
	case "facebook", "facebook_url":
		return "facebook"
	case "instagram", "instagram_url", "ig":
		return "instagram"
	case "youtube", "youtube_url", "youtu", "youtu_be":
		return "youtube"
	case "tiktok", "tiktok_url":
		return "tiktok"
	default:
		return ""
	}
}

func hostMatchesAllowed(host string) (string, bool) {
	host = strings.ToLower(strings.Trim(strings.TrimSpace(host), "."))
	if host == "" {
		return "", false
	}
	for domain, platform := range allowedSocialDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return platform, true
		}
	}
	return "", false
}

func sanitizeURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty url")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, errors.New("invalid url")
	}
	u.Scheme = "https"
	return u, nil
}

func stripTracking(u *url.URL) {
	if u == nil {
		return
	}
	query := u.Query()
	changed := false
	for key := range query {
		if strings.HasPrefix(strings.ToLower(key), trackingPrefix) {
			query.Del(key)
			changed = true
		}
	}
	if changed {
		u.RawQuery = query.Encode()
	}
}

func normalizePhone(raw, region string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if region == "" {
		region = defaultPhoneRegion
	}
	number, err := phonenumbers.Parse(raw, region)
	if err != nil {
		return ""
	}
	if !phonenumbers.IsPossibleNumber(number) || !phonenumbers.IsValidNumber(number) {
		return ""
	}
	return phonenumbers.Format(number, phonenumbers.E164)
}

func selectBestAddress(addresses []string) string {
	var best string
	var bestScore int
	for _, raw := range addresses {
		addr := strings.TrimSpace(raw)
		if addr == "" {
			continue
		}
		score := addressScore(addr)
		if score > bestScore {
			bestScore = score
			best = addr
		}
	}
	return best
}

func addressScore(addr string) int {
	segments := strings.FieldsFunc(addr, func(r rune) bool { return r == ',' || r == ';' })
	completeness := len(segments)
	lengthScore := len([]rune(addr))
	return completeness*1000 + lengthScore
}

func isDomainValid(domain string) bool {
	if strings.Count(domain, ".") == 0 {
		return false
	}
	parts := strings.Split(domain, ".")
	for _, part := range parts {
		if part == "" || strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return false
		}
	}
	return true
}

type systemDNSResolver struct{}

func (systemDNSResolver) LookupMX(ctx context.Context, domain string) ([]*net.MX, error) {
	return net.DefaultResolver.LookupMX(ctx, domain)
}
