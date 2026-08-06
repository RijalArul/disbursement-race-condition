package response

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/logger"
)

// MapError is the single place where a domain error becomes an HTTP status.
// Handlers and middleware hand their error here instead of choosing a status
// themselves, so the same domain condition can never answer 409 in one route
// and 400 in another.
//
// The full error chain — including any wrapping context and internal cause —
// goes to the server log, correlated by request_id. The response body only ever
// carries the message the domain layer marked as client-safe.
//
// Note on domain.ErrIdempotencyReplay: it is deliberately absent from the table
// below. A replay is a success, not a failure — the idempotency middleware
// answers it by writing back the stored response together with the
// X-Idempotent-Replayed header. Mapping it here would turn a correct replay
// into an HTTP error.
func MapError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	status, code, sentinel := classify(err)
	log := logger.FromCtx(c.Request.Context())

	if status >= http.StatusInternalServerError {
		log.Error("unhandled error", slog.Any("error", err))
		Err(c, status, code, "internal server error")
		c.Abort()
		return
	}

	log.Warn("request failed", slog.String("code", code), slog.Any("error", err))

	message := domain.ClientMessage(err)
	if message == "" && sentinel != nil {
		// A bare sentinel, or one wrapped only with fmt.Errorf. Its own text is
		// vague but safe; the wrapping context stays in the log where it
		// belongs rather than being echoed back to the caller.
		message = sentinel.Error()
	}

	Err(c, status, code, message)
	c.Abort()
}

// classify reports the HTTP status, the stable error code, and the sentinel
// that matched. The sentinel is returned so the caller has a safe fallback
// message when the error carries no client-facing text of its own.
func classify(err error) (int, string, error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return http.StatusBadRequest, "VALIDATION_ERROR", domain.ErrInvalidInput
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized, "UNAUTHORIZED", domain.ErrUnauthorized
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden, "FORBIDDEN", domain.ErrForbidden
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, "NOT_FOUND", domain.ErrNotFound
	case errors.Is(err, domain.ErrIdempotencyBusy):
		// Checked before ErrConflict: both answer 409, and if one ever ends up
		// wrapped inside the other this order decides which code wins.
		return http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS", domain.ErrIdempotencyBusy
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict, "CONFLICT", domain.ErrConflict
	case errors.Is(err, domain.ErrIdempotencyBodyMismatch):
		return http.StatusUnprocessableEntity, "IDEMPOTENCY_KEY_REUSED", domain.ErrIdempotencyBodyMismatch
	case errors.Is(err, domain.ErrRateLimited):
		return http.StatusTooManyRequests, "RATE_LIMITED", domain.ErrRateLimited
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR", nil
	}
}
