package ratelimit

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	authconst "github.com/RijalArul/disbursement-race-condition/internal/constants/auth"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware/common"
)

// userKey derives the bucket key for an authenticated request from the JWT
// identity Auth middleware already placed on context, not from IP.
func userKey(c *gin.Context) (string, bool) {
	id, ok := common.IdentityFromCtx(c.Request.Context())
	if !ok {
		return "", false
	}
	return "user:" + id.UserID, true
}

// ipKey derives the bucket key for the caller's IP.
func ipKey(c *gin.Context) string {
	return "ip:" + c.ClientIP()
}

// usernameKey peeks the login request body for its username without consuming
// the stream, so the handler's own ShouldBindJSON still works afterward.
// Normalized (trim+lowercase) so case variants of the same account share one bucket.
func usernameKey(c *gin.Context) (string, bool) {
	var req authconst.LoginRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil || req.Username == "" {
		return "", false
	}
	return "username:" + strings.ToLower(strings.TrimSpace(req.Username)), true
}
