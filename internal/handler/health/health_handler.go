package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	healthconst "github.com/RijalArul/disbursement-race-condition/internal/constants/health"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/logger"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
)

// pingTimeout bounds the database check. /health is polled by orchestrators on
// a short interval, so a hung database must fail the probe quickly rather than
// tie the request up until the client gives up.
const pingTimeout = 2 * time.Second

// Pinger is the slice of *sql.DB this handler needs, kept narrow so tests can
// fake a reachable and an unreachable database without a live server.
type Pinger interface {
	PingContext(ctx context.Context) error
}

type Handler struct {
	db Pinger
}

func NewHandler(db Pinger) *Handler {
	return &Handler{db: db}
}

// Check reports whether the API can reach its database.
//
//	@Summary	Health check
//	@Tags		health
//	@Produce	json
//	@Success	200	{object}	response.Envelope{data=health.Response}
//	@Header		200	{string}	X-Request-ID		"Request correlation ID; echoed from the request header or generated if absent"
//	@Failure	503	{object}	response.Envelope	"DATABASE_UNAVAILABLE: the API is up but cannot reach its database"
//	@Router		/health [get]
func (h *Handler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), pingTimeout)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		// The client learns the database is unreachable; why it is unreachable
		// stays in the server log.
		logger.FromCtx(ctx).Error("health check: database ping failed", slog.String("error", err.Error()))
		response.Err(c, http.StatusServiceUnavailable, healthconst.ErrCodeDatabaseUnavailable, healthconst.DatabaseUnreachable)
		return
	}

	response.OK(c, http.StatusOK, healthconst.Response{
		Status:   healthconst.StatusOK,
		Database: healthconst.DatabaseUp,
	})
}
