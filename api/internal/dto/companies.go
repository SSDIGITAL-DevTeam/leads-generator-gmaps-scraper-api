package dto

// ListFilter contains query parameters for company listing endpoints.
type ListFilter struct {
	Q            string
	TypeBusiness string
	City         string
	Country      string
	MinRating    *float64
	Page         int
	PerPage      int
}
