package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/RijalArul/disbursement-race-condition/internal/constants"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
	"github.com/RijalArul/disbursement-race-condition/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req constants.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.MapError(c, domain.Invalid("invalid request body"))
		return
	}

	tokens, err := h.auth.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		response.MapError(c, err)
		return
	}

	response.OK(c, http.StatusOK, constants.TokenResponse{AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req constants.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.MapError(c, domain.Invalid("invalid request body"))
		return
	}

	tokens, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.MapError(c, err)
		return
	}

	response.OK(c, http.StatusOK, constants.TokenResponse{AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req constants.RefreshRequest
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
