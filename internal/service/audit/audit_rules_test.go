package audit

import (
	"errors"
	"testing"

	auditconst "github.com/RijalArul/disbursement-race-condition/internal/constants/audit"
	respconst "github.com/RijalArul/disbursement-race-condition/internal/constants/response"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
)

func TestValidateListDefaults(t *testing.T) {
	got, err := validateList(auditconst.ListRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Page != respconst.DefaultPage {
		t.Errorf("Page = %d, want %d", got.Page, respconst.DefaultPage)
	}
	if got.Limit != respconst.DefaultLimit {
		t.Errorf("Limit = %d, want %d", got.Limit, respconst.DefaultLimit)
	}
	if got.SortBy != "created_at" || got.SortOrder != "desc" {
		t.Errorf("sort = %q %q, want created_at desc", got.SortBy, got.SortOrder)
	}
}

func TestValidateListLimitClampedAtMax(t *testing.T) {
	got, err := validateList(auditconst.ListRequest{Limit: "500"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Limit != respconst.MaxLimit {
		t.Errorf("Limit = %d, want clamped to %d", got.Limit, respconst.MaxLimit)
	}
}

func TestValidateListAction(t *testing.T) {
	tests := []struct {
		name    string
		action  string
		wantErr bool
	}{
		{"created is allowed", auditconst.ActionCreated, false},
		{"status_changed is allowed", auditconst.ActionStatusChanged, false},
		{"deleted is allowed", auditconst.ActionDeleted, false},
		{"empty is allowed (no filter)", "", false},
		{"unknown action is rejected", "approved", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateList(auditconst.ListRequest{Action: tt.action})
			if tt.wantErr && !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got %v", err)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestValidateListSortByWhitelist(t *testing.T) {
	_, err := validateList(auditconst.ListRequest{SortBy: "actor_id"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for a non-whitelisted sort_by, got %v", err)
	}
}

func TestValidateListDateRangeInvalid(t *testing.T) {
	_, err := validateList(auditconst.ListRequest{DateFrom: "2026-08-10", DateTo: "2026-08-01"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
	if msg := domain.ClientMessage(err); msg != auditconst.DateRangeInvalid {
		t.Errorf("client message = %q, want %q", msg, auditconst.DateRangeInvalid)
	}
}

func TestValidateListInvalidDateFormat(t *testing.T) {
	_, err := validateList(auditconst.ListRequest{DateFrom: "10/08/2026"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
	if msg := domain.ClientMessage(err); msg != auditconst.InvalidDateFrom {
		t.Errorf("client message = %q, want %q", msg, auditconst.InvalidDateFrom)
	}
}
