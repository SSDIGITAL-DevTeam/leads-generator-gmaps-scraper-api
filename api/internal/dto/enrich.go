package dto

// EnrichResultRequest represents the payload sent by the worker after crawling a website.
type EnrichResultRequest struct {
	CompanyID      string              `json:"company_id"`
	Emails         []string            `json:"emails"`
	Phones         []string            `json:"phones"`
	Socials        map[string][]string `json:"socials"`
	Address        *string             `json:"address"`
	ContactFormURL *string             `json:"contact_form_url"`
	AboutSummary   *string             `json:"about_summary"`
	Website        string              `json:"website"`
	PagesCrawled   int                 `json:"pages_crawled"`
}
