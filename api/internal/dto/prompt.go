package dto

// PromptSearchRequest represents a free-form search prompt.
type PromptSearchRequest struct {
	Prompt    string  `json:"prompt"`
	Country   string  `json:"country,omitempty"`
	MinRating float64 `json:"min_rating,omitempty"`
	Limit     int     `json:"limit,omitempty"`
}

// PromptSearchResponse echoes the interpreted parameters from the prompt.
type PromptSearchResponse struct {
	Prompt       string  `json:"prompt"`
	TypeBusiness string  `json:"type_business"`
	City         string  `json:"city"`
	Country      string  `json:"country"`
	MinRating    float64 `json:"min_rating,omitempty"`
	Limit        int     `json:"limit,omitempty"`
	RequireNoWebsite bool `json:"require_no_website"`
}
