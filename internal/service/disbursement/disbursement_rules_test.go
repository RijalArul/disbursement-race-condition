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
