package entity

import (
	"time"

	"github.com/google/uuid"
)

// CompanyEnrichment stores supplemental contact details scraped from company websites.
type CompanyEnrichment struct {
	CompanyID      uuid.UUID            `json:"company_id"`
	Emails         []string             `json:"emails"`
	Phones         []string             `json:"phones"`
	Socials        map[string][]string  `json:"socials"`
	Address        *string              `json:"address"`
	ContactFormURL *string              `json:"contact_form_url"`
	AboutSummary   *string              `json:"about_summary"`
	Metadata       map[string]any       `json:"metadata"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}
