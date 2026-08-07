package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
)

// openIntegrationDB connects to a live Postgres using the same env vars
// config.Config reads (DB_HOST, DB_PORT, ...). It skips the test rather than
// failing when the required vars aren't set, so `go test ./...` stays green
// on a machine without `docker-compose up postgres migrate` running first.
func openIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()

	host := os.Getenv("DB_HOST")
	if host == "" {
		t.Skip("DB_HOST not set; skipping integration test (run `docker-compose up -d postgres migrate` and set DB_* env vars to enable)")
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host,
		getenvDefault("DB_PORT", "5432"),
		getenvDefault("DB_USER", "postgres"),
		getenvDefault("DB_PASSWORD", "postgres"),
		getenvDefault("DB_NAME", "disbursement"),
		getenvDefault("DB_SSLMODE", "disable"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to integration database: %v", err)
	}
	return db
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// seedUser inserts a minimal user row so disbursements.created_by's foreign
// key has something real to point at, and returns the new user's ID.
func seedUser(t *testing.T, db *gorm.DB) string {
	t.Helper()

	u := &models.User{
		ID:           uuid.NewString(),
		Username:     "race-test-" + uuid.NewString(),
		PasswordHash: "not-a-real-hash",
		Role:         domain.RoleOperator,
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return u.ID
}

// seedPendingDisbursement inserts a PENDING row via the repository's own
// Create path (so IDs come from the real DSB-000001 sequence) and returns its ID.
func seedPendingDisbursement(t *testing.T, db *gorm.DB, repo DisbursementRepository) string {
	t.Helper()

	d := &models.Disbursement{
		RecipientName: "Race Condition Test",
		AccountNumber: "1234567890",
		BankCode:      "BCA",
		Amount:        1_250_000,
		AdminFee:      2500,
		Status:        domain.StatusPending,
		CreatedBy:     seedUser(t, db),
	}
	if err := repo.Create(context.Background(), d); err != nil {
		t.Fatalf("failed to seed disbursement: %v", err)
	}
	return d.ID
}

// TestUpdateStatusConcurrentApprovalIsRace fires two concurrent UpdateStatus
// calls (APPROVED vs REJECTED) at the same PENDING row. The SELECT ... FOR
// UPDATE lock in UpdateStatus must serialize them so exactly one succeeds and
// the other observes the row already left PENDING (ErrConflict) — proving
// the disbursement can never end up double-approved.
func TestUpdateStatusConcurrentApprovalIsRace(t *testing.T) {
	db := openIntegrationDB(t)
	repo := NewDisbursementRepository(db)
	id := seedPendingDisbursement(t, db, repo)

	approver := seedUser(t, db)
	rejecter := seedUser(t, db)

	var wg sync.WaitGroup
	results := make([]error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _, err := repo.UpdateStatus(context.Background(), id, domain.StatusApproved, approver, nil)
		results[0] = err
	}()
	go func() {
		defer wg.Done()
		_, _, err := repo.UpdateStatus(context.Background(), id, domain.StatusRejected, rejecter, nil)
		results[1] = err
	}()
	wg.Wait()

	successes, conflicts := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, domain.ErrConflict):
			conflicts++
		default:
			t.Fatalf("unexpected error from concurrent UpdateStatus: %v", err)
		}
	}

	if successes != 1 {
		t.Errorf("expected exactly 1 successful transition, got %d", successes)
	}
	if conflicts != 1 {
		t.Errorf("expected exactly 1 ErrConflict from the loser, got %d", conflicts)
	}

	final, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to read back final state: %v", err)
	}
	if final.Status != domain.StatusApproved && final.Status != domain.StatusRejected {
		t.Fatalf("final status = %q, want APPROVED or REJECTED (never left PENDING or corrupted)", final.Status)
	}
}
