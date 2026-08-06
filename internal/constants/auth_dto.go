package constants

// DTOs for the auth endpoints (AUTH-01..03). Kept out of internal/handler so
// request/response shapes are defined in one place, separate from binding logic.

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
