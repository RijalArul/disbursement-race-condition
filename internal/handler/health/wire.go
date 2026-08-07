package health

import (
	"database/sql"

	"github.com/gin-gonic/gin"
)

// New builds the health handler over the pool that serves the rest of the API,
// so the probe exercises the same connections real requests use.
func New(db *sql.DB) *Handler {
	return NewHandler(db)
}

// RegisterRoutes attaches GET /health. It stays outside the JWT middleware and
// outside the rate limiter: an orchestrator polling liveness has no token, and
// throttling the probe would turn a busy minute into a false restart.
func RegisterRoutes(r *gin.Engine, h *Handler) {
	r.GET("/health", h.Check)
}
