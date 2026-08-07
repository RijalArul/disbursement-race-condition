package common

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/RijalArul/disbursement-race-condition/internal/pkg/logger"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
)

// Recovery converts a panic in any handler into a 500 JSON envelope instead
// of a crashed process, and logs the panic with its stack trace and
// request_id correlation so the server log alone is enough to debug it.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log := logger.FromCtx(c.Request.Context())
				log.Error("panic recovered",
					slog.Any("panic", r),
					slog.String("stack", string(debug.Stack())),
				)
				response.Err(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
				c.Abort()
			}
		}()
		c.Next()
	}
}
