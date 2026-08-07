package idempotency

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware/common"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/hash"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// fakeRepo stands in for the database. It stores rows in memory keyed by
// user_id+key, mirroring the UNIQUE(user_id, idempotency_key) constraint.
type fakeRepo struct {
	rows        map[string]*models.IdempotencyKey
	reserveErr  error
	completeErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{rows: make(map[string]*models.IdempotencyKey)}
}

func rowKey(userID, key string) string { return userID + "|" + key }

func (f *fakeRepo) TryReserve(ctx context.Context, userID, key, requestHash string) (bool, error) {
	if f.reserveErr != nil {
		return false, f.reserveErr
	}
	rk := rowKey(userID, key)
	if existing, exists := f.rows[rk]; exists && existing.ExpiresAt.After(time.Now()) {
		return false, nil
	}
	f.rows[rk] = &models.IdempotencyKey{
		UserID:         userID,
		IdempotencyKey: key,
		RequestHash:    requestHash,
		State:          domain.IdemStateProcessing,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}
	return true, nil
}

func (f *fakeRepo) Find(ctx context.Context, userID, key string) (*models.IdempotencyKey, error) {
	row, ok := f.rows[rowKey(userID, key)]
	if !ok {
		return nil, domain.NotFound("idempotency key not found")
	}
	return row, nil
}

func (f *fakeRepo) Complete(ctx context.Context, userID, key string, status int, body []byte) error {
	if f.completeErr != nil {
		return f.completeErr
	}
	row := f.rows[rowKey(userID, key)]
	row.State = domain.IdemStateCompleted
	row.ResponseStatus = &status
	row.ResponseBody = body
	return nil
}

func (f *fakeRepo) Delete(ctx context.Context, userID, key string) error {
	delete(f.rows, rowKey(userID, key))
	return nil
}

const testUserID = "11111111-1111-1111-1111-111111111111"

func setupRouter(repo *fakeRepo, handler gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := common.WithIdentity(c.Request.Context(), testUserID, "operator01", string(domain.RoleOperator))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.POST("/disbursements", Middleware(repo), handler)
	return r
}

func createdHandler(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"id": "DSB-000001"}})
}

func doPost(r *gin.Engine, key, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/disbursements", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestIdempotencyMissingHeaderPassesThrough(t *testing.T) {
	repo := newFakeRepo()
	r := setupRouter(repo, createdHandler)

	w := doPost(r, "", `{"amount":1000}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}
	if len(repo.rows) != 0 {
		t.Errorf("expected no idempotency row without a header, got %d", len(repo.rows))
	}
}

func TestIdempotencyMalformedKeyRejected(t *testing.T) {
	repo := newFakeRepo()
	r := setupRouter(repo, createdHandler)

	w := doPost(r, "not-a-uuid", `{"amount":1000}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed key, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestIdempotencyFirstRequestRunsHandlerAndStores(t *testing.T) {
	repo := newFakeRepo()
	r := setupRouter(repo, createdHandler)
	key := uuid.NewString()

	w := doPost(r, key, `{"amount":1000}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if w.Header().Get("X-Idempotent-Replayed") != "" {
		t.Error("first request must not carry the replay header")
	}

	row := repo.rows[rowKey(testUserID, key)]
	if row == nil {
		t.Fatal("expected a stored row after the first request")
	}
	if row.State != domain.IdemStateCompleted {
		t.Errorf("state = %q, want COMPLETED", row.State)
	}
	if row.ResponseStatus == nil || *row.ResponseStatus != http.StatusCreated {
		t.Errorf("stored response status = %v, want 201", row.ResponseStatus)
	}
}

func TestIdempotencyReplaySameBodyReturnsStoredResponse(t *testing.T) {
	repo := newFakeRepo()
	r := setupRouter(repo, createdHandler)
	key := uuid.NewString()
	body := `{"amount":1000}`

	first := doPost(r, key, body)
	second := doPost(r, key, body)

	if second.Code != first.Code {
		t.Errorf("replay status = %d, want %d", second.Code, first.Code)
	}
	if second.Body.String() != first.Body.String() {
		t.Errorf("replay body = %q, want %q", second.Body.String(), first.Body.String())
	}
	if second.Header().Get("X-Idempotent-Replayed") != "true" {
		t.Error("replay must carry X-Idempotent-Replayed: true")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(second.Body.Bytes(), &payload); err != nil {
		t.Fatalf("replay body is not valid JSON: %v", err)
	}
}

func TestIdempotencyReplayDifferentBodyRejected(t *testing.T) {
	repo := newFakeRepo()
	r := setupRouter(repo, createdHandler)
	key := uuid.NewString()

	doPost(r, key, `{"amount":1000}`)
	second := doPost(r, key, `{"amount":2000}`)

	if second.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for reused key with different body, got %d (body: %s)", second.Code, second.Body.String())
	}
}

func TestIdempotencyConcurrentRequestStillProcessingReturns409(t *testing.T) {
	repo := newFakeRepo()
	key := uuid.NewString()
	body := `{"amount":1000}`
	// Simulate a first request that reserved the key but has not completed yet.
	repo.rows[rowKey(testUserID, key)] = &models.IdempotencyKey{
		UserID:         testUserID,
		IdempotencyKey: key,
		RequestHash:    hash.HashToken(body),
		State:          domain.IdemStateProcessing,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}
	r := setupRouter(repo, createdHandler)

	w := doPost(r, key, body)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 while the first request is still processing, got %d", w.Code)
	}
}

// Per the ticket's own decision order, a body-hash mismatch is checked before the
// PROCESSING state, so a differing body on a still-running request answers 422, not 409.
func TestIdempotencyBodyMismatchWinsOverBusy(t *testing.T) {
	repo := newFakeRepo()
	key := uuid.NewString()
	repo.rows[rowKey(testUserID, key)] = &models.IdempotencyKey{
		UserID:         testUserID,
		IdempotencyKey: key,
		RequestHash:    hash.HashToken(`{"amount":1000}`),
		State:          domain.IdemStateProcessing,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}
	r := setupRouter(repo, createdHandler)

	w := doPost(r, key, `{"amount":9999}`)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for a differing body even while processing, got %d", w.Code)
	}
}

const otherUserID = "22222222-2222-2222-2222-222222222222"

func setupRouterForUser(repo *fakeRepo, userID string, handler gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := common.WithIdentity(c.Request.Context(), userID, "operator01", string(domain.RoleOperator))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.POST("/disbursements", Middleware(repo), handler)
	return r
}

func failingHandler(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, gin.H{"success": false})
}

func TestIdempotencyServiceFailureDeletesReservationAllowingRetry(t *testing.T) {
	repo := newFakeRepo()
	key := uuid.NewString()
	body := `{"amount":1000}`

	failingRouter := setupRouter(repo, failingHandler)
	first := doPost(failingRouter, key, body)

	if first.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from failing handler, got %d (body: %s)", first.Code, first.Body.String())
	}
	if _, exists := repo.rows[rowKey(testUserID, key)]; exists {
		t.Fatal("expected reservation to be deleted after a 500 response")
	}

	retryRouter := setupRouter(repo, createdHandler)
	second := doPost(retryRouter, key, body)

	if second.Code != http.StatusCreated {
		t.Fatalf("expected retry with same key to be treated as fresh (201), got %d (body: %s)", second.Code, second.Body.String())
	}
}

func TestIdempotencyKeyScopedPerUser(t *testing.T) {
	repo := newFakeRepo()
	key := uuid.NewString()
	body := `{"amount":1000}`

	firstRouter := setupRouterForUser(repo, testUserID, createdHandler)
	first := doPost(firstRouter, key, body)

	secondRouter := setupRouterForUser(repo, otherUserID, createdHandler)
	second := doPost(secondRouter, key, body)

	if first.Code != http.StatusCreated {
		t.Fatalf("expected 201 for first user, got %d (body: %s)", first.Code, first.Body.String())
	}
	if second.Code != http.StatusCreated {
		t.Fatalf("expected 201 for second user (key scoped per user), got %d (body: %s)", second.Code, second.Body.String())
	}

	if _, exists := repo.rows[rowKey(testUserID, key)]; !exists {
		t.Error("expected a row scoped to testUserID")
	}
	if _, exists := repo.rows[rowKey(otherUserID, key)]; !exists {
		t.Error("expected a row scoped to otherUserID")
	}
	if len(repo.rows) != 2 {
		t.Errorf("expected 2 distinct rows for the same key across users, got %d", len(repo.rows))
	}
}

func TestIdempotencyExpiredKeyTreatedAsNew(t *testing.T) {
	repo := newFakeRepo()
	key := uuid.NewString()
	repo.rows[rowKey(testUserID, key)] = &models.IdempotencyKey{
		UserID:         testUserID,
		IdempotencyKey: key,
		RequestHash:    "somehash",
		State:          domain.IdemStateCompleted,
		ExpiresAt:      time.Now().Add(-time.Hour),
	}
	r := setupRouter(repo, createdHandler)

	w := doPost(r, key, `{"amount":1000}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for a key whose prior reservation expired, got %d (body: %s)", w.Code, w.Body.String())
	}
}
