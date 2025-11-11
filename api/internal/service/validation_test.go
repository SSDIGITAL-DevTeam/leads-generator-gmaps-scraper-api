package service

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestCleanEmailsValidatesSyntaxAndMX(t *testing.T) {
	resolver := &stubDNSResolver{
		mx: map[string]bool{
			"example.com": true,
		},
	}
	p := NewDataProcessor("US", WithDNSResolver(resolver), WithHTTPClient(&noopHTTPClient{}))

	emails := []string{
		"Test@Example.com",
		"test@example.com",
		"invalid@",
		"user@missingmx.com",
	}

	got := p.cleanEmails(context.Background(), emails)
	if len(got) != 1 || got[0] != "test@example.com" {
		t.Fatalf("expected only normalized valid email, got %#v", got)
	}
}

func TestNormalizePhonesDeduplicatesAcrossPrimaryAndSecondary(t *testing.T) {
	p := NewDataProcessor("US", WithHTTPClient(&noopHTTPClient{}))
	phones := p.normalizePhones(" (415) 555-1234 ", []string{"+14155551234", "12345"})

	if len(phones) != 1 || phones[0] != "+14155551234" {
		t.Fatalf("unexpected normalized phones: %#v", phones)
	}
}

func TestValidateSocialsEnforcesDomainAndResolution(t *testing.T) {
	httpClient := &stubHTTPClient{
		responses: map[string]int{
			"HEAD https://www.linkedin.com/company/test-company": http.StatusOK,
			"HEAD https://facebook.com/page":                     http.StatusMethodNotAllowed,
			"GET https://facebook.com/page":                      http.StatusOK,
		},
	}
	p := NewDataProcessor("US", WithDNSResolver(&stubDNSResolver{}), WithHTTPClient(httpClient))

	input := map[string][]string{
		"linkedin":  {"https://www.linkedin.com/company/test-company?utm_source=newsletter"},
		"facebook":  {"http://facebook.com/page"},
		"instagram": {"https://example.com/not-allowed"},
	}

	result := p.validateSocials(context.Background(), input)

	if result.LinkedIn != "https://www.linkedin.com/company/test-company" {
		t.Fatalf("linkedin not cleaned correctly: %s", result.LinkedIn)
	}
	if result.Facebook != "https://facebook.com/page" {
		t.Fatalf("facebook fallback HEAD/GET failed: %s", result.Facebook)
	}
	if result.Instagram != "" {
		t.Fatalf("instagram from disallowed domain should be empty, got %s", result.Instagram)
	}
}

func TestSelectBestAddressPrefersMostComplete(t *testing.T) {
	addresses := []string{
		"Short address",
		"Longer address, Suite 101",
		"",
		"Another, Address, Line, City",
	}

	best := selectBestAddress(addresses)
	if best != "Another, Address, Line, City" {
		t.Fatalf("unexpected best address: %s", best)
	}
}

func TestProcessReturnsStructuredOutput(t *testing.T) {
	resolver := &stubDNSResolver{
		mx: map[string]bool{"example.com": true},
	}
	httpClient := &stubHTTPClient{
		responses: map[string]int{
			"HEAD https://www.linkedin.com/company/test": http.StatusOK,
		},
	}
	p := NewDataProcessor("US", WithDNSResolver(resolver), WithHTTPClient(httpClient))

	payload := RawEnrichedData{
		CompanyID:       "123",
		Emails:          []string{"USER@EXAMPLE.COM"},
		PrimaryPhone:    "4155551234",
		SecondaryPhones: []string{"+14155551234"},
		SocialLinks: map[string][]string{
			"linkedin": {"https://www.linkedin.com/company/test?utm_medium=feed"},
		},
		Addresses:      []string{"Short", "123 Main St, Springfield, CA 94107"},
		ContactFormURL: "company.com/contact?utm_source=ads",
	}

	result, err := p.Process(context.Background(), payload)
	if err != nil {
		t.Fatalf("process returned error: %v", err)
	}
	if result.CompanyID != "123" {
		t.Fatalf("unexpected company id %s", result.CompanyID)
	}
	if len(result.Emails) != 1 || result.Emails[0] != "user@example.com" {
		t.Fatalf("emails not cleaned: %#v", result.Emails)
	}
	if len(result.Phones) != 1 || result.Phones[0] != "+14155551234" {
		t.Fatalf("phones not normalized: %#v", result.Phones)
	}
	if result.Socials.LinkedIn != "https://www.linkedin.com/company/test" {
		t.Fatalf("linkedin sanitized incorrectly: %s", result.Socials.LinkedIn)
	}
	if result.Address != "123 Main St, Springfield, CA 94107" {
		t.Fatalf("address selection incorrect: %s", result.Address)
	}
	if result.ContactFormURL != "https://company.com/contact" {
		t.Fatalf("contact form not sanitized: %s", result.ContactFormURL)
	}
}

type stubDNSResolver struct {
	mx map[string]bool
}

func (s *stubDNSResolver) LookupMX(_ context.Context, domain string) ([]*net.MX, error) {
	if s.mx == nil {
		return nil, errors.New("no mx")
	}
	if ok := s.mx[domain]; ok {
		return []*net.MX{{Host: "mail." + domain, Pref: 10}}, nil
	}
	return nil, errors.New("no mx")
}

type stubHTTPClient struct {
	responses map[string]int
}

func (c *stubHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c.responses == nil {
		return nil, errors.New("no response configured")
	}
	key := req.Method + " " + req.URL.String()
	status, ok := c.responses[key]
	if !ok {
		status = http.StatusNotFound
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

type noopHTTPClient struct{}

func (n *noopHTTPClient) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("http client disabled for test")
}
