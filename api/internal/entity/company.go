package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Company represents a business stored in the catalogue.
type Company struct {
	ID           uuid.UUID       `json:"id"`
	PlaceID      *string         `json:"place_id,omitempty"`
	ScrapeRunID  *uuid.UUID      `json:"scrape_run_id,omitempty"`
	Company      string          `json:"company"`
	Phone        *string         `json:"phone,omitempty"`
	Website      *string         `json:"website,omitempty"`
	Rating       *float64        `json:"rating,omitempty"`
	Reviews      *int            `json:"reviews,omitempty"`
	TypeBusiness *string         `json:"type_business,omitempty"`
	Address      *string         `json:"address,omitempty"`
	City         *string         `json:"city,omitempty"`
	Country      *string         `json:"country,omitempty"`
	Longitude    *float64        `json:"longitude,omitempty"`
	Latitude     *float64        `json:"latitude,omitempty"`
	Raw          json.RawMessage `json:"raw"`
	ScrapedAt    *time.Time      `json:"scraped_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
