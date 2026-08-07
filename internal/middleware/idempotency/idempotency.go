// Package idempotency implements DSB-02: reserve-then-complete idempotency for POST /disbursements.
package idempotency

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/validate"
	idemrepo "github.com/RijalArul/disbursement-race-condition/internal/repository/idempotency"
)

const keyHeader = "Idempotency-Key"

// bodyCapture tees everything written to the client into buf so it can be persisted after the handler runs.
type bodyCapture struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w *bodyCapture) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

// reservation is what the request needs before it can claim or replay a key.
type reservation struct {
	userID      string
	key         string
	requestHash string
}

// prepareRequest validates the key, resolves the caller, and hashes the body — everything
// needed to reserve or replay, without touching the repository.
func prepareRequest(c *gin.Context, key string) (reservation, error) {
	if !validate.RequiredUUID(key) {
		return reservation{}, domain.Invalid("Idempotency-Key must be a valid UUID")
	}

	userID, err := requireIdentity(c)
	if err != nil {
		return reservation{}, err
	}

	requestHash, err := readRequestHash(c)
	if err != nil {
		return reservation{}, err
	}

	return reservation{userID: userID, key: key, requestHash: requestHash}, nil
}

// Middleware makes the guarded route safe to retry: a repeated request with the same
// Idempotency-Key and body within 24h replays the first response instead of running twice.
func Middleware(repo idemrepo.IdempotencyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader(keyHeader)
		if key == "" {
			c.Next()
			return
		}

		res, err := prepareRequest(c, key)
		if err != nil {
			response.MapError(c, err)
			return
		}

		runReserved(c, repo, res)
	}
}

// runReserved claims the key and either runs the handler (first request) or replays the stored response.
func runReserved(c *gin.Context, repo idemrepo.IdempotencyRepository, res reservation) {
	ctx := c.Request.Context()

	reserved, err := repo.TryReserve(ctx, res.userID, res.key, res.requestHash)
	if err != nil {
		response.MapError(c, err)
		return
	}

	if !reserved {
		replayExisting(c, repo, res)
		return
	}

	runAndStore(c, repo, res)
}

// runAndStore executes the handler once, then persists its response for replay, or deletes
// the reservation on failure so the client can retry with the same key.
func runAndStore(c *gin.Context, repo idemrepo.IdempotencyRepository, res reservation) {
	ctx := c.Request.Context()

	capture := &bodyCapture{ResponseWriter: c.Writer, buf: &bytes.Buffer{}}
	c.Writer = capture

	c.Next()

	status := c.Writer.Status()
	if status >= http.StatusInternalServerError || len(c.Errors) > 0 {
		if delErr := repo.Delete(ctx, res.userID, res.key); delErr != nil {
			response.MapError(c, delErr)
		}
		return
	}

	_ = repo.Complete(ctx, res.userID, res.key, status, capture.buf.Bytes())
}

// replayExisting decides the response for a key already reserved by an earlier request.
func replayExisting(c *gin.Context, repo idemrepo.IdempotencyRepository, res reservation) {
	row, err := repo.Find(c.Request.Context(), res.userID, res.key)
	if err != nil {
		response.MapError(c, err)
		return
	}

	if err := validateReplay(row, res.requestHash); err != nil {
		response.MapError(c, err)
		return
	}

	c.Header("X-Idempotent-Replayed", "true")
	c.Data(*row.ResponseStatus, "application/json; charset=utf-8", row.ResponseBody)
	c.Abort()
}
