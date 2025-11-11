package dto

import (
	"time"

	"github.com/google/uuid"
)

// ListFilter contains query parameters for company listing endpoints.
type ListFilter struct {
	Q             string
	TypeBusiness  string
	City          string
	Country       string
	MinRating     *float64
	UpdatedSince  *time.Time
	ScrapeRunID   *uuid.UUID
	Sort          string
	LatestRunOnly bool
	Page          int
	PerPage       int
	Limit         int
	WebsiteStatus string
}
