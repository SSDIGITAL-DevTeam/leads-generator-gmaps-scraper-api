package entity

import (
	"time"

	"github.com/google/uuid"
)

// WebsiteEnrichedContact stores normalized contact details for a company website.
type WebsiteEnrichedContact struct {
	ID             uuid.UUID `json:"id"`
	CompanyID      uuid.UUID `json:"company_id"`
	Emails         []string  `json:"emails"`
	Phones         []string  `json:"phones"`
	LinkedInURL    *string   `json:"linkedin_url,omitempty"`
	FacebookURL    *string   `json:"facebook_url,omitempty"`
	InstagramURL   *string   `json:"instagram_url,omitempty"`
	YouTubeURL     *string   `json:"youtube_url,omitempty"`
	TikTokURL      *string   `json:"tiktok_url,omitempty"`
	Address        *string   `json:"address,omitempty"`
	ContactFormURL *string   `json:"contact_form_url,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
