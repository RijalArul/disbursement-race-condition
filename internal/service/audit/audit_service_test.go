package audit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	auditconst "github.com/RijalArul/disbursement-race-condition/internal/constants/audit"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/worker"
)

// fakeAuditRepo stands in for the database, with an optional hook so tests
// can be notified exactly when Create runs — Enqueue hands off to a worker
// goroutine, so assertions need a synchronization point rather than a sleep.
type fakeAuditRepo struct {
	mu       sync.Mutex
	created  []*models.AuditLog
	createFn func(*models.AuditLog)
	createErr error

	listRows  []models.AuditLog
	listTotal int64
	listErr   error
	listQuery auditconst.ListQuery
}

func (f *fakeAuditRepo) Create(ctx context.Context, a *models.AuditLog) error {
	f.mu.Lock()
	f.created = append(f.created, a)
	f.mu.Unlock()
	if f.createFn != nil {
		f.createFn(a)
	}
	return f.createErr
}

func (f *fakeAuditRepo) List(ctx context.Context, q auditconst.ListQuery) ([]models.AuditLog, int64, error) {
	f.listQuery = q
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	return f.listRows, f.listTotal, nil
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestServiceEnqueueRecordsAfterCommit(t *testing.T) {
	repo := &fakeAuditRepo{}
	pool := worker.NewPool(1, 4, silentLogger())
	defer pool.Shutdown()
	svc := NewService(repo, pool)

	var wg sync.WaitGroup
	wg.Add(1)
	repo.createFn = func(*models.AuditLog) { wg.Done() }

	ctx := middleware.WithRequestID(context.Background(), "req-123")
	svc.Enqueue(ctx, Event{
		ActorID:  "user-1",
		Action:   auditconst.ActionCreated,
		EntityID: "DSB-000001",
		Before:   nil,
		After:    map[string]string{"status": "PENDING"},
	})

	wg.Wait()

	if len(repo.created) != 1 {
		t.Fatalf("Create called %d times, want 1", len(repo.created))
	}
	got := repo.created[0]
	if got.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", got.RequestID, "req-123")
	}
	if got.Action != auditconst.ActionCreated {
		t.Errorf("Action = %q, want %q", got.Action, auditconst.ActionCreated)
	}
	if string(got.Before) != "null" {
		t.Errorf("Before = %s, want the JSON null literal for action=created", got.Before)
	}
	var after map[string]string
	if err := json.Unmarshal(got.After, &after); err != nil {
		t.Fatalf("After did not unmarshal: %v", err)
	}
	if after["status"] != "PENDING" {
		t.Errorf("After.status = %q, want PENDING", after["status"])
	}
}

func TestServiceEnqueueRepositoryFailureDoesNotPanic(t *testing.T) {
	repo := &fakeAuditRepo{createErr: errors.New("insert failed")}
	pool := worker.NewPool(1, 4, silentLogger())
	defer pool.Shutdown()
	svc := NewService(repo, pool)

	var wg sync.WaitGroup
	wg.Add(1)
	repo.createFn = func(*models.AuditLog) { wg.Done() }

	svc.Enqueue(context.Background(), Event{Action: auditconst.ActionDeleted, EntityID: "DSB-000002"})

	wg.Wait()
}

func TestServiceList(t *testing.T) {
	repo := &fakeAuditRepo{listRows: []models.AuditLog{{ID: "LOG-000001"}}, listTotal: 1}
	pool := worker.NewPool(1, 4, silentLogger())
	defer pool.Shutdown()
	svc := NewService(repo, pool)

	rows, meta, err := svc.List(context.Background(), auditconst.ListRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("rows = %v, want 1 row", rows)
	}
	if meta.Total != 1 {
		t.Errorf("Total = %d, want 1", meta.Total)
	}
}

func TestServiceListValidationFailureDoesNotQuery(t *testing.T) {
	repo := &fakeAuditRepo{}
	pool := worker.NewPool(1, 4, silentLogger())
	defer pool.Shutdown()
	svc := NewService(repo, pool)

	_, _, err := svc.List(context.Background(), auditconst.ListRequest{Action: "bogus"})
	if err == nil {
		t.Fatal("expected a validation error, got nil")
	}
	if repo.listQuery != (auditconst.ListQuery{}) {
		t.Error("repository was queried on validation failure")
	}
}
