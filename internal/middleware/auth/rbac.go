package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
)

// RequireRole rejects callers whose role from the JWT (set by Auth) is not
// in allowed. Must run after Auth. Insufficient role is 403, never 401 —
// the caller is authenticated, just not authorized for this action.
func RequireRole(allowed ...domain.UserRole) gin.HandlerFunc {
	allowedSet := make(map[domain.UserRole]bool, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = true
	}

	return func(c *gin.Context) {
		role, _ := c.Get(ContextKeyRole)
		roleStr, _ := role.(string)

		if !allowedSet[domain.UserRole(roleStr)] {
			response.Err(c, http.StatusForbidden, "FORBIDDEN", "insufficient role")
			c.Abort()
			return
		}

		c.Next()
	}
}
