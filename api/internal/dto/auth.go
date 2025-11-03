package dto

// LoginRequest captures credential input.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse contains the issued access token.
type LoginResponse struct {
	AccessToken string `json:"access_token"`
}
