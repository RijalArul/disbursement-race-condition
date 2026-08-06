package disbursement

import (
	"errors"
	"testing"

	disbconst "github.com/RijalArul/disbursement-race-condition/internal/constants/disbursement"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
)

func TestCalculateAdminFee(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		want   int64
	}{
		{"minimum allowed amount", 10_000, disbconst.AdminFeeLow},
		{"well below threshold", 1_250_000, disbconst.AdminFeeLow},
		{"one below threshold", 4_999_999, disbconst.AdminFeeLow},
		{"exactly at threshold takes the higher fee", 5_000_000, disbconst.AdminFeeHigh},
		{"one above threshold", 5_000_001, disbconst.AdminFeeHigh},
		{"far above threshold", 950_000_000, disbconst.AdminFeeHigh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateAdminFee(tt.amount); got != tt.want {
				t.Errorf("CalculateAdminFee(%d) = %d, want %d", tt.amount, got, tt.want)
			}
		})
	}
}

func TestValidateCreate(t *testing.T) {
	valid := func() disbconst.CreateInput {
		return disbconst.CreateInput{
			RecipientName: "Budi Santoso",
			AccountNumber: "1234567890",
			BankCode:      "BCA",
			Amount:        1_250_000,
		}
	}

	tests := []struct {
		name        string
		mutate      func(*disbconst.CreateInput)
		wantErr     bool
		wantMessage string
	}{
		{
			name:   "valid input passes",
			mutate: func(*disbconst.CreateInput) {},
		},
		{
			name:        "missing recipient_name",
			mutate:      func(in *disbconst.CreateInput) { in.RecipientName = "" },
			wantErr:     true,
			wantMessage: disbconst.RecipientNameRequired,
		},
		{
			name:        "whitespace-only recipient_name is treated as missing",
			mutate:      func(in *disbconst.CreateInput) { in.RecipientName = "   " },
			wantErr:     true,
			wantMessage: disbconst.RecipientNameRequired,
		},
		{
			name:        "missing account_number",
			mutate:      func(in *disbconst.CreateInput) { in.AccountNumber = "" },
			wantErr:     true,
			wantMessage: disbconst.AccountNumberRequired,
		},
		{
			name:        "missing bank_code",
			mutate:      func(in *disbconst.CreateInput) { in.BankCode = "" },
			wantErr:     true,
			wantMessage: disbconst.BankCodeRequired,
		},
		{
			name:   "unknown bank_code is accepted: presence is all that is checked",
			mutate: func(in *disbconst.CreateInput) { in.BankCode = "NOT-A-REAL-BANK" },
		},
		{
			name:   "amount exactly at the minimum is accepted",
			mutate: func(in *disbconst.CreateInput) { in.Amount = 10_000 },
		},
		{
			name:        "amount one below the minimum is rejected",
			mutate:      func(in *disbconst.CreateInput) { in.Amount = 9_999 },
			wantErr:     true,
			wantMessage: disbconst.AmountBelowMinimum,
		},
		{
			name:        "zero amount is rejected",
			mutate:      func(in *disbconst.CreateInput) { in.Amount = 0 },
			wantErr:     true,
			wantMessage: disbconst.AmountBelowMinimum,
		},
		{
			name:        "negative amount is rejected",
			mutate:      func(in *disbconst.CreateInput) { in.Amount = -1_000_000 },
			wantErr:     true,
			wantMessage: disbconst.AmountBelowMinimum,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := valid()
			tt.mutate(&in)

			got, err := validateCreate(in)

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got %v", err)
			}
			if msg := domain.ClientMessage(err); msg != tt.wantMessage {
				t.Errorf("client message = %q, want %q", msg, tt.wantMessage)
			}
			_ = got
		})
	}
}

func TestValidateCreateTrimsWhitespace(t *testing.T) {
	in := disbconst.CreateInput{
		RecipientName: "  Budi Santoso  ",
		AccountNumber: "  1234567890  ",
		BankCode:      "  BCA  ",
		Amount:        10_000,
	}

	got, err := validateCreate(in)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.RecipientName != "Budi Santoso" {
		t.Errorf("RecipientName = %q, want %q", got.RecipientName, "Budi Santoso")
	}
	if got.AccountNumber != "1234567890" {
		t.Errorf("AccountNumber = %q, want %q", got.AccountNumber, "1234567890")
	}
	if got.BankCode != "BCA" {
		t.Errorf("BankCode = %q, want %q", got.BankCode, "BCA")
	}
}

func TestValidateListDefaults(t *testing.T) {
	got, err := validateList(disbconst.ListRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Page != disbconst.DefaultPage {
		t.Errorf("Page = %d, want %d", got.Page, disbconst.DefaultPage)
	}
	if got.Limit != disbconst.DefaultLimit {
		t.Errorf("Limit = %d, want %d", got.Limit, disbconst.DefaultLimit)
	}
	if got.SortBy != "created_at" {
		t.Errorf("SortBy = %q, want created_at", got.SortBy)
	}
	if got.SortOrder != "desc" {
		t.Errorf("SortOrder = %q, want desc", got.SortOrder)
	}
}

func TestValidateListLimitBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		limit     string
		wantErr   bool
		wantLimit int
	}{
		{"exactly at max is accepted as-is", "100", false, 100},
		{"one above max is clamped", "101", false, disbconst.MaxLimit},
		{"far above max is clamped", "9999", false, disbconst.MaxLimit},
		{"zero is rejected", "0", true, 0},
		{"negative is rejected", "-5", true, 0},
		{"non-numeric is rejected", "abc", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateList(disbconst.ListRequest{Limit: tt.limit})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				if msg := domain.ClientMessage(err); msg != disbconst.InvalidLimit {
					t.Errorf("client message = %q, want %q", msg, disbconst.InvalidLimit)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got.Limit != tt.wantLimit {
				t.Errorf("Limit = %d, want %d", got.Limit, tt.wantLimit)
			}
		})
	}
}

func TestValidateListDateRange(t *testing.T) {
	tests := []struct {
		name        string
		dateFrom    string
		dateTo      string
		wantErr     bool
		wantMessage string
	}{
		{"from before to is accepted", "2026-08-01", "2026-08-10", false, ""},
		{"from equal to is accepted", "2026-08-01", "2026-08-01", false, ""},
		{"from after to is rejected", "2026-08-10", "2026-08-01", true, disbconst.DateRangeInvalid},
		{"malformed from is rejected", "2026/08/01", "2026-08-10", true, disbconst.InvalidDateFrom},
		{"malformed to is rejected", "2026-08-01", "10-08-2026", true, disbconst.InvalidDateTo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateList(disbconst.ListRequest{DateFrom: tt.dateFrom, DateTo: tt.dateTo})
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if msg := domain.ClientMessage(err); msg != tt.wantMessage {
				t.Errorf("client message = %q, want %q", msg, tt.wantMessage)
			}
		})
	}
}

func TestValidateListSortWhitelist(t *testing.T) {
	tests := []struct {
		name        string
		sortBy      string
		sortOrder   string
		wantErr     bool
		wantMessage string
	}{
		{"created_at is allowed", "created_at", "asc", false, ""},
		{"amount is allowed", "amount", "desc", false, ""},
		{"status is allowed", "status", "asc", false, ""},
		{"unknown column is rejected", "internal_notes", "asc", true, disbconst.InvalidSortBy},
		{"sql injection attempt in sort_by is rejected", "id; DROP TABLE disbursements;--", "asc", true, disbconst.InvalidSortBy},
		{"unknown sort_order is rejected", "amount", "sideways", true, disbconst.InvalidSortOrder},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateList(disbconst.ListRequest{SortBy: tt.sortBy, SortOrder: tt.sortOrder})
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if msg := domain.ClientMessage(err); msg != tt.wantMessage {
				t.Errorf("client message = %q, want %q", msg, tt.wantMessage)
			}
		})
	}
}

func TestValidateListStatusWhitelist(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		wantErr bool
	}{
		{"empty is accepted (no filter)", "", false},
		{"PENDING is accepted", "PENDING", false},
		{"APPROVED is accepted", "APPROVED", false},
		{"REJECTED is accepted", "REJECTED", false},
		{"FAILED is accepted", "FAILED", false},
		{"lowercase is rejected", "pending", true},
		{"unknown status is rejected", "CANCELLED", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateList(disbconst.ListRequest{Status: tt.status})
			if tt.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestValidateUpdateStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		wantErr bool
	}{
		{"APPROVED is accepted", "APPROVED", false},
		{"REJECTED is accepted", "REJECTED", false},
		{"PENDING is rejected", "PENDING", true},
		{"FAILED is rejected", "FAILED", true},
		{"lowercase is rejected", "approved", true},
		{"empty is rejected", "", true},
		{"unknown value is rejected", "CANCELLED", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUpdateStatus(tt.status)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				if !errors.Is(err, domain.ErrInvalidInput) {
					t.Errorf("expected ErrInvalidInput, got %v", err)
				}
				if msg := domain.ClientMessage(err); msg != disbconst.InvalidStatusTransition {
					t.Errorf("client message = %q, want %q", msg, disbconst.InvalidStatusTransition)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestValidateListSearchIsTrimmed(t *testing.T) {
	got, err := validateList(disbconst.ListRequest{Search: "  Budi  "})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Search != "Budi" {
		t.Errorf("Search = %q, want %q", got.Search, "Budi")
	}
}
