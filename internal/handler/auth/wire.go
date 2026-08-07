package auth

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/RijalArul/disbursement-race-condition/internal/pkg/jwt"
	authrepo "github.com/RijalArul/disbursement-race-condition/internal/repository/auth"
	authsvc "github.com/RijalArul/disbursement-race-condition/internal/service/auth"
)

// New assembles the auth module's repositories, service, and handler.
func New(db *gorm.DB, issuer *jwt.Issuer, refreshTTL time.Duration) *Handler {
	users := authrepo.NewUserRepository(db)
	refreshTokens := authrepo.NewRefreshTokenRepository(db)
	svc := authsvc.NewService(users, refreshTokens, issuer, refreshTTL)
	return NewHandler(svc)
}

// RegisterRoutes attaches POST /auth/login, /auth/refresh, /auth/logout.
func RegisterRoutes(r *gin.Engine, h *Handler) {
	group := r.Group("/auth")
	group.POST("/login", h.Login)
	group.POST("/refresh", h.Refresh)
	group.POST("/logout", h.Logout)
}
