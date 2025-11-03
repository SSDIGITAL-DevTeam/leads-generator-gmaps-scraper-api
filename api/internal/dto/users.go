package dto

// RegisterRequest captures self-service registration payloads.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// CreateUserRequest is used by administrators to create new users.
type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// UpdateUserRequest captures administrator-triggered partial updates.
type UpdateUserRequest struct {
	Email    *string `json:"email,omitempty"`
	Password *string `json:"password,omitempty"`
	Role     *string `json:"role,omitempty"`
}

// UserResponse represents user data returned to clients.
type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}
