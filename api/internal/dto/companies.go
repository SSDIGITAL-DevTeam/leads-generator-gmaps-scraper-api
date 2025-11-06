package dto

import "time"

// ListFilter contains query parameters for company listing endpoints.
type ListFilter struct {
	Q             string
	TypeBusiness  string
	City          string
	Country       string
	MinRating     *float64
	UpdatedSince  *time.Time
	Sort          string
	LatestRunOnly bool
	Page          int
	PerPage       int
}
