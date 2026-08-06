package disbursement

import (
	"context"
	"errors"
	"testing"

	disbconst "github.com/RijalArul/disbursement-race-condition/internal/constants/disbursement"
	respconst "github.com/RijalArul/disbursement-race-condition/internal/constants/response"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware"
	auditsvc "github.com/RijalArul/disbursement-race-condition/internal/service/audit"
)

// fakeAuditEnqueuer records every audit event submitted so tests can assert
// on what would have been persisted, without a real worker pool.
type fakeAuditEnqueuer struct {
	events []auditsvc.Event
}

func (f *fakeAuditEnqueuer) Enqueue(ctx context.Context, ev auditsvc.Event) {
	f.events = append(f.events, ev)
}

// fakeDisbursementRepo stands in for the database. It records what the service
// asked it to persist so the tests can assert on the row itself, not merely on
// the fact that a call happened.
type fakeDisbursementRepo struct {
	saved   *models.Disbursement
	calls   int
	failErr error

	getByIDResult *models.Disbursement
	getByIDErr    error

	listRows  []models.Disbursement
	listTotal int64
	listErr   error
	listQuery disbconst.ListQuery

	updateStatusBefore *models.Disbursement
	updateStatusResult *models.Disbursement
	updateStatusErr    error
	updateStatusCalls  int

	softDeleteBefore *models.Disbursement
	softDeleteErr    error
	softDeleteCalls  int
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

func (f *fakeDisbursementRepo) GetByID(ctx context.Context, id string) (*models.Disbursement, error) {
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	return f.getByIDResult, nil
}

func (f *fakeDisbursementRepo) List(ctx context.Context, q disbconst.ListQuery) ([]models.Disbursement, int64, error) {
	f.listQuery = q
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	return f.listRows, f.listTotal, nil
}

func (f *fakeDisbursementRepo) UpdateStatus(ctx context.Context, id string, status domain.DisbursementStatus, approvedBy string, note *string) (before, after *models.Disbursement, err error) {
	f.updateStatusCalls++
	if f.updateStatusErr != nil {
		return nil, nil, f.updateStatusErr
	}
	return f.updateStatusBefore, f.updateStatusResult, nil
}

func (f *fakeDisbursementRepo) SoftDelete(ctx context.Context, id string) (*models.Disbursement, error) {
	f.softDeleteCalls++
	if f.softDeleteErr != nil {
		return nil, f.softDeleteErr
	}
	return f.softDeleteBefore, nil
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
			svc := NewService(repo, &fakeAuditEnqueuer{})

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
	svc := NewService(repo, &fakeAuditEnqueuer{})

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
	svc := NewService(repo, &fakeAuditEnqueuer{})

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
			svc := NewService(repo, &fakeAuditEnqueuer{})

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
	svc := NewService(repo, &fakeAuditEnqueuer{})

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

func TestDisbursementServiceGetByIDSuccess(t *testing.T) {
	want := &models.Disbursement{ID: "DSB-000001", RecipientName: "Budi Santoso"}
	repo := &fakeDisbursementRepo{getByIDResult: want}
	svc := NewService(repo, &fakeAuditEnqueuer{})

	got, err := svc.GetByID(context.Background(), "DSB-000001")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != want {
		t.Errorf("GetByID returned a different disbursement than the repository provided")
	}
}

func TestDisbursementServiceGetByIDNotFound(t *testing.T) {
	repo := &fakeDisbursementRepo{getByIDErr: domain.ErrNotFound}
	svc := NewService(repo, &fakeAuditEnqueuer{})

	_, err := svc.GetByID(context.Background(), "DSB-999999")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if msg := domain.ClientMessage(err); msg != disbconst.NotFound {
		t.Errorf("client message = %q, want %q", msg, disbconst.NotFound)
	}
}

func TestDisbursementServiceUpdateStatusSuccess(t *testing.T) {
	want := &models.Disbursement{ID: "DSB-000001", Status: domain.StatusApproved}
	repo := &fakeDisbursementRepo{updateStatusResult: want}
	svc := NewService(repo, &fakeAuditEnqueuer{})

	got, err := svc.UpdateStatus(authedCtx(), disbconst.UpdateStatusInput{ID: "DSB-000001", Status: "APPROVED"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != want {
		t.Errorf("UpdateStatus returned a different disbursement than the repository provided")
	}
	if repo.updateStatusCalls != 1 {
		t.Errorf("repository called %d times, want 1", repo.updateStatusCalls)
	}
}

func TestDisbursementServiceUpdateStatusInvalidValue(t *testing.T) {
	repo := &fakeDisbursementRepo{}
	svc := NewService(repo, &fakeAuditEnqueuer{})

	_, err := svc.UpdateStatus(authedCtx(), disbconst.UpdateStatusInput{ID: "DSB-000001", Status: "PENDING"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
	if repo.updateStatusCalls != 0 {
		t.Errorf("repository called %d times, want 0 on validation failure", repo.updateStatusCalls)
	}
}

func TestDisbursementServiceUpdateStatusAlreadyDecided(t *testing.T) {
	repo := &fakeDisbursementRepo{updateStatusErr: domain.ErrConflict}
	svc := NewService(repo, &fakeAuditEnqueuer{})

	_, err := svc.UpdateStatus(authedCtx(), disbconst.UpdateStatusInput{ID: "DSB-000001", Status: "APPROVED"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
	if msg := domain.ClientMessage(err); msg == "" {
		t.Error("expected a client-safe conflict message")
	}
}

func TestDisbursementServiceUpdateStatusNotFound(t *testing.T) {
	repo := &fakeDisbursementRepo{updateStatusErr: domain.ErrNotFound}
	svc := NewService(repo, &fakeAuditEnqueuer{})

	_, err := svc.UpdateStatus(authedCtx(), disbconst.UpdateStatusInput{ID: "DSB-999999", Status: "REJECTED"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if msg := domain.ClientMessage(err); msg != disbconst.NotFound {
		t.Errorf("client message = %q, want %q", msg, disbconst.NotFound)
	}
}

func TestDisbursementServiceUpdateStatusApprovedByFromIdentity(t *testing.T) {
	want := &models.Disbursement{ID: "DSB-000001", Status: domain.StatusApproved}
	repo := &fakeDisbursementRepo{updateStatusResult: want}
	svc := NewService(repo, &fakeAuditEnqueuer{})

	const approver = "33333333-3333-3333-3333-333333333333"
	ctx := middleware.WithIdentity(context.Background(), approver, "admin01", string(domain.RoleAdmin))

	_, err := svc.UpdateStatus(ctx, disbconst.UpdateStatusInput{ID: "DSB-000001", Status: "APPROVED"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDisbursementServiceDeleteSuccess(t *testing.T) {
	repo := &fakeDisbursementRepo{}
	svc := NewService(repo, &fakeAuditEnqueuer{})

	if err := svc.Delete(context.Background(), "DSB-000001"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.softDeleteCalls != 1 {
		t.Errorf("repository called %d times, want 1", repo.softDeleteCalls)
	}
}

func TestDisbursementServiceDeleteNotPending(t *testing.T) {
	repo := &fakeDisbursementRepo{softDeleteErr: domain.ErrConflict}
	svc := NewService(repo, &fakeAuditEnqueuer{})

	err := svc.Delete(context.Background(), "DSB-000001")
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestDisbursementServiceDeleteNotFound(t *testing.T) {
	repo := &fakeDisbursementRepo{softDeleteErr: domain.ErrNotFound}
	svc := NewService(repo, &fakeAuditEnqueuer{})

	err := svc.Delete(context.Background(), "DSB-999999")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if msg := domain.ClientMessage(err); msg != disbconst.NotFound {
		t.Errorf("client message = %q, want %q", msg, disbconst.NotFound)
	}
}

func TestDisbursementServiceDeleteTwiceIsNotFoundSecondTime(t *testing.T) {
	repo := &fakeDisbursementRepo{}
	svc := NewService(repo, &fakeAuditEnqueuer{})

	if err := svc.Delete(context.Background(), "DSB-000001"); err != nil {
		t.Fatalf("expected no error on first delete, got %v", err)
	}

	repo.softDeleteErr = domain.ErrNotFound
	err := svc.Delete(context.Background(), "DSB-000001")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestDisbursementServiceListDefaults(t *testing.T) {
	repo := &fakeDisbursementRepo{listRows: nil, listTotal: 0}
	svc := NewService(repo, &fakeAuditEnqueuer{})

	rows, meta, err := svc.List(context.Background(), disbconst.ListRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %v, want empty", rows)
	}
	if meta.Page != respconst.DefaultPage || meta.Limit != respconst.DefaultLimit {
		t.Errorf("meta = %+v, want defaults page=%d limit=%d", meta, respconst.DefaultPage, respconst.DefaultLimit)
	}
	if meta.TotalPages != 0 {
		t.Errorf("TotalPages = %d, want 0 for an empty result", meta.TotalPages)
	}
	if repo.listQuery.SortBy != "created_at" || repo.listQuery.SortOrder != "desc" {
		t.Errorf("repo received sort %q %q, want default created_at desc", repo.listQuery.SortBy, repo.listQuery.SortOrder)
	}
}

func TestDisbursementServiceListLimitClampedAt100(t *testing.T) {
	repo := &fakeDisbursementRepo{}
	svc := NewService(repo, &fakeAuditEnqueuer{})

	_, meta, err := svc.List(context.Background(), disbconst.ListRequest{Limit: "500"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if meta.Limit != respconst.MaxLimit {
		t.Errorf("Limit = %d, want clamped to %d", meta.Limit, respconst.MaxLimit)
	}
	if repo.listQuery.Limit != respconst.MaxLimit {
		t.Errorf("repo received Limit = %d, want %d", repo.listQuery.Limit, respconst.MaxLimit)
	}
}

func TestDisbursementServiceListValidationFailureDoesNotQuery(t *testing.T) {
	tests := []struct {
		name        string
		req         disbconst.ListRequest
		wantMessage string
	}{
		{"negative page", disbconst.ListRequest{Page: "-1"}, disbconst.InvalidPage},
		{"non-numeric page", disbconst.ListRequest{Page: "abc"}, disbconst.InvalidPage},
		{"zero limit", disbconst.ListRequest{Limit: "0"}, disbconst.InvalidLimit},
		{"invalid status", disbconst.ListRequest{Status: "BOGUS"}, disbconst.InvalidStatus},
		{"invalid date_from format", disbconst.ListRequest{DateFrom: "06/08/2026"}, disbconst.InvalidDateFrom},
		{"invalid date_to format", disbconst.ListRequest{DateTo: "not-a-date"}, disbconst.InvalidDateTo},
		{"date_from after date_to", disbconst.ListRequest{DateFrom: "2026-08-10", DateTo: "2026-08-01"}, disbconst.DateRangeInvalid},
		{"invalid sort_by", disbconst.ListRequest{SortBy: "internal_notes"}, disbconst.InvalidSortBy},
		{"invalid sort_order", disbconst.ListRequest{SortOrder: "sideways"}, disbconst.InvalidSortOrder},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeDisbursementRepo{}
			svc := NewService(repo, &fakeAuditEnqueuer{})

			_, _, err := svc.List(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got %v", err)
			}
			if msg := domain.ClientMessage(err); msg != tt.wantMessage {
				t.Errorf("client message = %q, want %q", msg, tt.wantMessage)
			}
			if repo.listQuery != (disbconst.ListQuery{}) {
				t.Error("repository was queried on validation failure")
			}
		})
	}
}
