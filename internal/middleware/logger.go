package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/RijalArul/disbursement-race-condition/internal/pkg/logger"
)

// AccessLog logs one structured line per request with request_id correlation.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status_code", c.Writer.Status()),
			slog.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
		}

		// Only authenticated requests carry a user. /auth/login and /health
		// legitimately have none, and emitting "user":"" there would read as
		// a user whose name is empty rather than as no user at all.
		if username := c.GetString(ContextKeyUsername); username != "" {
			attrs = append(attrs, slog.String("user", username))
		}

		logger.FromCtx(c.Request.Context()).Info("request completed", attrs...)
	}
}
