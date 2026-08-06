package auth

// DTOs for the auth endpoints (AUTH-01..03).

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Tokens is the service's own contract, kept separate from the wire-bound TokenResponse.
type Tokens struct {
	AccessToken  string
	RefreshToken string
}
