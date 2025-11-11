package scoring

import "testing"

func TestComputeScore_FullCoverage(t *testing.T) {
	input := LeadFeatures{
		Emails: []string{"info@example.com"},
		Phones: []string{"+123456789"},
		Socials: map[string]string{
			"LinkedIn":  "https://linkedin.com/company/acme",
			"instagram": "https://instagram.com/acme",
			"facebook":  "https://facebook.com/acme",
			"youtube":   "https://youtube.com/@acme,https://youtube.com/@acme-demo",
			"other":     "https://twitter.com/acme|https://twitter.com/acme2|https://twitter.com/acme3|https://twitter.com/acme4|https://twitter.com/acme5|https://twitter.com/acme6",
		},
		HasHTTPS:       true,
		HasContactPage: true,
		HasAboutPage:   true,
		HasContactForm: true,
		Address:        "123 Main Street, Springfield, US",
		Website:        "https://acme.com",
	}

	score := ComputeScore(input)

	if score.Total != 100 {
		t.Fatalf("expected full score 100, got %d", score.Total)
	}
	if score.Breakdown[categoryContact] != 30 {
		t.Fatalf("expected contact completeness 30, got %d", score.Breakdown[categoryContact])
	}
	if score.Breakdown[categoryWebsite] != 30 {
		t.Fatalf("expected website quality 30, got %d", score.Breakdown[categoryWebsite])
	}
	if score.Breakdown[categorySocial] != 20 {
		t.Fatalf("expected social presence 20, got %d", score.Breakdown[categorySocial])
	}
	if score.Breakdown[categoryBusiness] != 20 {
		t.Fatalf("expected business profile 20, got %d", score.Breakdown[categoryBusiness])
	}
}

func TestComputeScore_MinimalSignals(t *testing.T) {
	input := LeadFeatures{
		Emails: []string{"   "},
		Phones: []string{},
		Socials: map[string]string{
			"linkedin": "",
		},
		Website: "http://myshop.wordpress.com",
		Address: "Jl. Merdeka",
	}

	score := ComputeScore(input)

	if score.Total != 0 {
		t.Fatalf("expected zero score for insufficient signals, got %d", score.Total)
	}
	if score.Breakdown[categoryBusiness] != 0 {
		t.Fatalf("expected business profile 0, got %d", score.Breakdown[categoryBusiness])
	}
}

func TestHighQualityDomain(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"https://example.com", true},
		{"http://www.example.co.id", true},
		{"mybrand.wordpress.com", false},
		{"", false},
		{"ftp://subdomain.googlepages.com", false},
	}

	for _, tc := range cases {
		if got := highQualityDomain(tc.input); got != tc.want {
			t.Fatalf("highQualityDomain(%q)=%v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestHasCompleteAddress(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"123 Main St, Springfield, US", true},
		{" 456 High Road London ", false}, // no comma separator
		{"Somewhere", false},
		{"Jl. Merdeka No. 8, Jakarta", true},
	}

	for _, tc := range cases {
		if got := hasCompleteAddress(tc.input); got != tc.want {
			t.Fatalf("hasCompleteAddress(%q)=%v, want %v", tc.input, got, tc.want)
		}
	}
}
