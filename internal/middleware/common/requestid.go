package common

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/RijalArul/disbursement-race-condition/internal/pkg/logger"
)

const RequestIDHeader = "X-Request-ID"

// RequestID assigns/propagates a request id and attaches a request-scoped
// logger to the request context so every downstream log line correlates.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader(RequestIDHeader)
		if reqID == "" {
			reqID = uuid.NewString()
		}
		c.Writer.Header().Set(RequestIDHeader, reqID)

		ctx := logger.WithRequestID(c.Request.Context(), reqID)
		ctx = WithRequestID(ctx, reqID)
		c.Request = c.Request.WithContext(ctx)
		c.Set(ContextKeyRequestID, reqID)

		c.Next()
	}
}
