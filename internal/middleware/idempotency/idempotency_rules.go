package idempotency

import (
	"bytes"
	"io"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware/common"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/hash"
)

func requireUnexpired(row *models.IdempotencyKey) error {
	if row.ExpiresAt.Before(time.Now()) {
		return domain.Conflict("idempotency key expired mid-flight, retry")
	}
	return nil
}

func requireSameBody(row *models.IdempotencyKey, requestHash string) error {
	if row.RequestHash != requestHash {
		return domain.NewError(domain.ErrIdempotencyBodyMismatch, "idempotency key reused with a different request body")
	}
	return nil
}

func requireCompleted(row *models.IdempotencyKey) error {
	if row.State == domain.IdemStateProcessing {
		return domain.NewError(domain.ErrIdempotencyBusy, "request with this idempotency key is still being processed")
	}
	return nil
}

// validateReplay runs every rule a replay candidate must pass, in order.
func validateReplay(row *models.IdempotencyKey, requestHash string) error {
	if err := requireUnexpired(row); err != nil {
		return err
	}
	if err := requireSameBody(row, requestHash); err != nil {
		return err
	}
	return requireCompleted(row)
}

// requireIdentity resolves the caller Auth middleware attached to ctx.
func requireIdentity(c *gin.Context) (string, error) {
	identity, ok := common.IdentityFromCtx(c.Request.Context())
	if !ok {
		return "", domain.Unauthorized("authentication required")
	}
	return identity.UserID, nil
}

// readRequestHash hashes the request body and restores it so the handler can still read it.
func readRequestHash(c *gin.Context) (string, error) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return "", domain.Invalid("failed to read request body")
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return hash.HashToken(string(bodyBytes)), nil
}
