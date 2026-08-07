package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/RijalArul/disbursement-race-condition/internal/pkg/jwt"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
)

// Auth validates the access token on every route it guards. Any failure —
// missing header, malformed value, bad signature, or expiry — is a 401:
// these are authentication failures, distinct from the 403 an authenticated
// but under-privileged caller gets from RBAC.
func Auth(issuer *jwt.Issuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if header == "" || !strings.HasPrefix(header, prefix) {
			response.Err(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or malformed authorization header")
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(header, prefix)
		claims, err := issuer.Parse(tokenString)
		if err != nil {
			msg := "invalid token"
			if errors.Is(err, jwt.ErrExpiredToken) {
				msg = "token expired"
			}
			response.Err(c, http.StatusUnauthorized, "UNAUTHORIZED", msg)
			c.Abort()
			return
		}

		ctx := WithIdentity(c.Request.Context(), claims.UserID, claims.Username, string(claims.Role))
		c.Request = c.Request.WithContext(ctx)
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyRole, string(claims.Role))

		c.Next()
	}
}
