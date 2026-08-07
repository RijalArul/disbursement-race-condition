package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	authconst "github.com/RijalArul/disbursement-race-condition/internal/constants/auth"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
	authsvc "github.com/RijalArul/disbursement-race-condition/internal/service/auth"
)

type Handler struct {
	auth *authsvc.Service
}

func NewHandler(auth *authsvc.Service) *Handler {
	return &Handler{auth: auth}
}

// Login authenticates a user and issues an access/refresh token pair.
//
//	@Summary	Login
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		request	body		auth.LoginRequest	true	"Credentials"
//	@Success	200		{object}	response.Envelope{data=auth.TokenResponse}
//	@Header		200		{string}	X-Request-ID	"Request correlation ID; echoed from the request header or generated if absent"
//	@Failure	400		{object}	response.Envelope	"VALIDATION_ERROR: malformed body"
//	@Failure	401		{object}	response.Envelope	"UNAUTHORIZED: invalid username or password"
//	@Router		/auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req authconst.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.MapError(c, domain.Invalid("invalid request body"))
		return
	}

	tokens, err := h.auth.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		response.MapError(c, err)
		return
	}

	response.OK(c, http.StatusOK, authconst.TokenResponse{AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken})
}

// Refresh rotates a refresh token for a new access/refresh token pair.
//
//	@Summary	Refresh access token
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		request	body		auth.RefreshRequest	true	"Refresh token"
//	@Success	200		{object}	response.Envelope{data=auth.TokenResponse}
//	@Header		200		{string}	X-Request-ID	"Request correlation ID; echoed from the request header or generated if absent"
//	@Failure	400		{object}	response.Envelope	"VALIDATION_ERROR: malformed body"
//	@Failure	401		{object}	response.Envelope	"UNAUTHORIZED: token invalid, expired, revoked, or reused after rotation"
//	@Router		/auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	var req authconst.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.MapError(c, domain.Invalid("invalid request body"))
		return
	}

	tokens, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.MapError(c, err)
		return
	}

	response.OK(c, http.StatusOK, authconst.TokenResponse{AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken})
}

// Logout revokes a refresh token. Idempotent: an already-revoked or unknown token still returns 200.
//
//	@Summary	Logout
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		request	body		auth.RefreshRequest	true	"Refresh token"
//	@Success	200		{object}	response.Envelope{data=object{message=string}}
//	@Header		200		{string}	X-Request-ID	"Request correlation ID; echoed from the request header or generated if absent"
//	@Failure	400		{object}	response.Envelope	"VALIDATION_ERROR: malformed body"
//	@Router		/auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	var req authconst.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.MapError(c, domain.Invalid("invalid request body"))
		return
	}

	if err := h.auth.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.MapError(c, err)
		return
	}

	response.OK(c, http.StatusOK, gin.H{"message": "logged out"})
}
