package disbursement

import (
	"strings"

	disbconst "github.com/RijalArul/disbursement-race-condition/internal/constants/disbursement"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
)

// Business rules for disbursements (DSB-01). Pure — no receiver, no I/O — so
// they unit-test without a fake repository.

// CalculateAdminFee returns the admin fee for an amount. The comparison is
// >=, so an amount of exactly AdminFeeThreshold is charged the higher fee.
func CalculateAdminFee(amount int64) int64 {
	if amount >= disbconst.AdminFeeThreshold {
		return disbconst.AdminFeeHigh
	}
	return disbconst.AdminFeeLow
}

func trimCreateInput(in disbconst.CreateInput) disbconst.CreateInput {
	in.RecipientName = strings.TrimSpace(in.RecipientName)
	in.AccountNumber = strings.TrimSpace(in.AccountNumber)
	in.BankCode = strings.TrimSpace(in.BankCode)
	return in
}

func requireField(value, message string) error {
	if value == "" {
		return domain.Invalid(message)
	}
	return nil
}

// requireMinAmount also rejects zero and negative amounts — both are below the minimum.
func requireMinAmount(amount int64) error {
	if amount < disbconst.MinAmount {
		return domain.Invalid(disbconst.AmountBelowMinimum)
	}
	return nil
}

// validateCreate enforces the DSB-01 field rules and returns the input
// trimmed. bank_code is checked for presence only — no registry lookup.
func validateCreate(in disbconst.CreateInput) (disbconst.CreateInput, error) {
	in = trimCreateInput(in)

	if err := requireField(in.RecipientName, disbconst.RecipientNameRequired); err != nil {
		return in, err
	}
	if err := requireField(in.AccountNumber, disbconst.AccountNumberRequired); err != nil {
		return in, err
	}
	if err := requireField(in.BankCode, disbconst.BankCodeRequired); err != nil {
		return in, err
	}
	if err := requireMinAmount(in.Amount); err != nil {
		return in, err
	}

	return in, nil
}
