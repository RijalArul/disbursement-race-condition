package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	authconst "github.com/RijalArul/disbursement-race-condition/internal/constants/auth"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
	authsvc "github.com/RijalArul/disbursement-race-condition/internal/service/auth"
)

type AuthHandler struct {
	auth *authsvc.Service
}

func NewAuthHandler(auth *authsvc.Service) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Login(c *gin.Context) {
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

func (h *AuthHandler) Refresh(c *gin.Context) {
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

func (h *AuthHandler) Logout(c *gin.Context) {
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
