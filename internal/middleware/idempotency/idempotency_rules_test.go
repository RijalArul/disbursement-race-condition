package idempotency

import (
	"errors"
	"testing"
	"time"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
)

func freshRow() *models.IdempotencyKey {
	return &models.IdempotencyKey{
		RequestHash: "abc123",
		State:       domain.IdemStateCompleted,
		ExpiresAt:   time.Now().Add(time.Hour),
	}
}

func TestValidateReplay(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*models.IdempotencyKey)
		requestHash string
		wantErr     error
	}{
		{
			name:        "completed, same hash, unexpired",
			requestHash: "abc123",
			wantErr:     nil,
		},
		{
			name:        "expired",
			mutate:      func(r *models.IdempotencyKey) { r.ExpiresAt = time.Now().Add(-time.Minute) },
			requestHash: "abc123",
			wantErr:     domain.ErrConflict,
		},
		{
			name:        "different body",
			requestHash: "different-hash",
			wantErr:     domain.ErrIdempotencyBodyMismatch,
		},
		{
			name:        "still processing",
			mutate:      func(r *models.IdempotencyKey) { r.State = domain.IdemStateProcessing },
			requestHash: "abc123",
			wantErr:     domain.ErrIdempotencyBusy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := freshRow()
			if tt.mutate != nil {
				tt.mutate(row)
			}

			err := validateReplay(row, tt.requestHash)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

// Expiry must be checked before the hash mismatch — an expired key is treated as
// though it never existed, regardless of what body it was originally reserved with.
func TestValidateReplayChecksExpiryBeforeBodyMismatch(t *testing.T) {
	row := freshRow()
	row.ExpiresAt = time.Now().Add(-time.Minute)

	err := validateReplay(row, "different-hash")
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected expiry to win, got %v", err)
	}
}
