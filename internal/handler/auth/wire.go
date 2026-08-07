package auth

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/RijalArul/disbursement-race-condition/internal/middleware/ratelimit"
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
// login is rate-limited by IP+username, not user_id, since the caller has no
// JWT yet at this point.
func RegisterRoutes(r *gin.Engine, h *Handler, loginIPLimit, loginUsernameLimit int) {
	group := r.Group("/auth")
	group.POST("/login", ratelimit.PerLoginAttempt(loginIPLimit, loginUsernameLimit, time.Minute), h.Login)
	group.POST("/refresh", h.Refresh)
	group.POST("/logout", h.Logout)
}
