package dto

// ScrapeRequest is the payload used by the scraping endpoint.
type ScrapeRequest struct {
	TypeBusiness string  `json:"type_business"`
	Location     string  `json:"location,omitempty"`
	MinRating    float64 `json:"min_rating,omitempty"`
	City         string  `json:"city,omitempty"`
	Country      string  `json:"country,omitempty"`
}
