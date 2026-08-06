package disbursement

import (
	"context"
	"errors"
	"testing"

	disbconst "github.com/RijalArul/disbursement-race-condition/internal/constants/disbursement"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware"
)

// fakeDisbursementRepo stands in for the database. It records what the service
// asked it to persist so the tests can assert on the row itself, not merely on
// the fact that a call happened.
type fakeDisbursementRepo struct {
	saved   *models.Disbursement
	calls   int
	failErr error
}

func (f *fakeDisbursementRepo) Create(ctx context.Context, d *models.Disbursement) error {
	f.calls++
	if f.failErr != nil {
		return f.failErr
	}
	// Postgres assigns the id and timestamps; mimic that so callers see the
	// same shape they would in production.
	d.ID = "DSB-000001"
	f.saved = d
	return nil
}

const testUserID = "11111111-1111-1111-1111-111111111111"

func authedCtx() context.Context {
	return middleware.WithIdentity(context.Background(), testUserID, "operator01", string(domain.RoleOperator))
}

func validInput() disbconst.CreateInput {
	return disbconst.CreateInput{
		RecipientName: "Budi Santoso",
		AccountNumber: "1234567890",
		BankCode:      "BCA",
		Amount:        1_250_000,
	}
}

func TestDisbursementServiceCreateSuccess(t *testing.T) {
	note := "Pembayaran supplier"

	tests := []struct {
		name         string
		amount       int64
		note         *string
		wantAdminFee int64
	}{
		{"below fee threshold", 1_250_000, &note, disbconst.AdminFeeLow},
		{"one below fee threshold", 4_999_999, nil, disbconst.AdminFeeLow},
		{"exactly at fee threshold", 5_000_000, nil, disbconst.AdminFeeHigh},
		{"minimum allowed amount", 10_000, nil, disbconst.AdminFeeLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeDisbursementRepo{}
			svc := NewService(repo)

			in := validInput()
			in.Amount = tt.amount
			in.Note = tt.note

			got, err := svc.Create(authedCtx(), in)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if got.Amount != tt.amount {
				t.Errorf("Amount = %d, want %d", got.Amount, tt.amount)
			}
			if got.AdminFee != tt.wantAdminFee {
				t.Errorf("AdminFee = %d, want %d", got.AdminFee, tt.wantAdminFee)
			}
			if got.Status != domain.StatusPending {
				t.Errorf("Status = %q, want %q", got.Status, domain.StatusPending)
			}
			if got.ApprovedBy != nil {
				t.Errorf("ApprovedBy = %v, want nil on creation", *got.ApprovedBy)
			}
			if got.CreatedBy != testUserID {
				t.Errorf("CreatedBy = %q, want the authenticated user %q", got.CreatedBy, testUserID)
			}
			if got.BankCode != "BCA" {
				t.Errorf("BankCode = %q, want %q", got.BankCode, "BCA")
			}
			if got.ID == "" {
				t.Error("expected the persisted id to be returned")
			}

			if tt.note == nil {
				if got.Note != nil {
					t.Errorf("Note = %q, want nil when omitted", *got.Note)
				}
			} else if got.Note == nil || *got.Note != *tt.note {
				t.Errorf("Note = %v, want %q", got.Note, *tt.note)
			}

			if repo.saved == nil {
				t.Fatal("expected the disbursement to be persisted")
			}
			if repo.saved.AdminFee != tt.wantAdminFee {
				t.Errorf("persisted AdminFee = %d, want %d", repo.saved.AdminFee, tt.wantAdminFee)
			}
			if repo.saved.CreatedBy != testUserID {
				t.Errorf("persisted CreatedBy = %q, want %q", repo.saved.CreatedBy, testUserID)
			}
		})
	}
}

// The service must take created_by from the JWT identity on the context. There
// is no field on the input through which a caller could supply it, so this test
// pins that the value written is the authenticated one.
func TestDisbursementServiceCreatedByComesFromIdentity(t *testing.T) {
	repo := &fakeDisbursementRepo{}
	svc := NewService(repo)

	const otherUser = "22222222-2222-2222-2222-222222222222"
	ctx := middleware.WithIdentity(context.Background(), otherUser, "operator02", string(domain.RoleOperator))

	got, err := svc.Create(ctx, validInput())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.CreatedBy != otherUser {
		t.Errorf("CreatedBy = %q, want %q", got.CreatedBy, otherUser)
	}
}

func TestDisbursementServiceCreateWithoutIdentity(t *testing.T) {
	repo := &fakeDisbursementRepo{}
	svc := NewService(repo)

	_, err := svc.Create(context.Background(), validInput())
	if err == nil {
		t.Fatal("expected an error when no identity is on the context")
	}
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
	if repo.calls != 0 {
		t.Errorf("repository was called %d times, want 0 — nothing may be written without an identity", repo.calls)
	}
}

func TestDisbursementServiceCreateValidationFailureDoesNotPersist(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*disbconst.CreateInput)
		wantMessage string
	}{
		{
			name:        "amount below minimum",
			mutate:      func(in *disbconst.CreateInput) { in.Amount = 9_999 },
			wantMessage: disbconst.AmountBelowMinimum,
		},
		{
			name:        "missing bank_code",
			mutate:      func(in *disbconst.CreateInput) { in.BankCode = "" },
			wantMessage: disbconst.BankCodeRequired,
		},
		{
			name:        "missing recipient_name",
			mutate:      func(in *disbconst.CreateInput) { in.RecipientName = "" },
			wantMessage: disbconst.RecipientNameRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeDisbursementRepo{}
			svc := NewService(repo)

			in := validInput()
			tt.mutate(&in)

			_, err := svc.Create(authedCtx(), in)
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got %v", err)
			}
			if msg := domain.ClientMessage(err); msg != tt.wantMessage {
				t.Errorf("client message = %q, want %q", msg, tt.wantMessage)
			}
			if repo.calls != 0 {
				t.Errorf("repository was called %d times, want 0 on validation failure", repo.calls)
			}
		})
	}
}

// A repository failure must surface as an error, and the internal cause must
// stay reachable for the log while carrying nothing client-safe of its own.
func TestDisbursementServiceCreateRepositoryFailure(t *testing.T) {
	dbErr := errors.New("insert violates check constraint")
	repo := &fakeDisbursementRepo{failErr: dbErr}
	svc := NewService(repo)

	_, err := svc.Create(authedCtx(), validInput())
	if err == nil {
		t.Fatal("expected an error when the repository fails")
	}
	if !errors.Is(err, dbErr) {
		t.Errorf("expected the repository error to be wrapped, got %v", err)
	}
	if msg := domain.ClientMessage(err); msg != "" {
		t.Errorf("client message = %q, want empty so the mapper answers 500 with a generic body", msg)
	}
}
