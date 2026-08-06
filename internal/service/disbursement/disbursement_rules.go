package disbursement

import (
	"strconv"
	"strings"
	"time"

	disbconst "github.com/RijalArul/disbursement-race-condition/internal/constants/disbursement"
	respconst "github.com/RijalArul/disbursement-race-condition/internal/constants/response"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
)

// buildPageMeta computes total_pages, rounding up so a partial last page still counts.
func buildPageMeta(page, limit int, total int64) respconst.PageMeta {
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	return respconst.PageMeta{Page: page, Limit: limit, Total: total, TotalPages: totalPages}
}

const dateLayout = "2006-01-02"

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

func parsePage(raw string) (int, error) {
	if raw == "" {
		return disbconst.DefaultPage, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, domain.Invalid(disbconst.InvalidPage)
	}
	return n, nil
}

func parseLimit(raw string) (int, error) {
	if raw == "" {
		return disbconst.DefaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, domain.Invalid(disbconst.InvalidLimit)
	}
	if n > disbconst.MaxLimit {
		n = disbconst.MaxLimit
	}
	return n, nil
}

func parseStatus(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	switch domain.DisbursementStatus(raw) {
	case domain.StatusPending, domain.StatusApproved, domain.StatusRejected, domain.StatusFailed:
		return raw, nil
	default:
		return "", domain.Invalid(disbconst.InvalidStatus)
	}
}

func parseDate(raw, invalidMsg string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(dateLayout, raw)
	if err != nil {
		return nil, domain.Invalid(invalidMsg)
	}
	return &t, nil
}

func parseSortBy(raw string) (string, error) {
	if raw == "" {
		return "created_at", nil
	}
	if !domain.AllowedSortFields[raw] {
		return "", domain.Invalid(disbconst.InvalidSortBy)
	}
	return raw, nil
}

func parseSortOrder(raw string) (string, error) {
	if raw == "" {
		return "desc", nil
	}
	if raw != "asc" && raw != "desc" {
		return "", domain.Invalid(disbconst.InvalidSortOrder)
	}
	return raw, nil
}

// validateList parses and validates every GET /disbursements query param,
// returning a typed ListQuery ready for the repository.
func validateList(in disbconst.ListRequest) (disbconst.ListQuery, error) {
	page, err := parsePage(in.Page)
	if err != nil {
		return disbconst.ListQuery{}, err
	}
	limit, err := parseLimit(in.Limit)
	if err != nil {
		return disbconst.ListQuery{}, err
	}
	status, err := parseStatus(in.Status)
	if err != nil {
		return disbconst.ListQuery{}, err
	}
	dateFrom, err := parseDate(in.DateFrom, disbconst.InvalidDateFrom)
	if err != nil {
		return disbconst.ListQuery{}, err
	}
	dateTo, err := parseDate(in.DateTo, disbconst.InvalidDateTo)
	if err != nil {
		return disbconst.ListQuery{}, err
	}
	if dateFrom != nil && dateTo != nil && dateFrom.After(*dateTo) {
		return disbconst.ListQuery{}, domain.Invalid(disbconst.DateRangeInvalid)
	}
	sortBy, err := parseSortBy(in.SortBy)
	if err != nil {
		return disbconst.ListQuery{}, err
	}
	sortOrder, err := parseSortOrder(in.SortOrder)
	if err != nil {
		return disbconst.ListQuery{}, err
	}

	return disbconst.ListQuery{
		PageQuery: respconst.PageQuery{
			Page:      page,
			Limit:     limit,
			SortBy:    sortBy,
			SortOrder: sortOrder,
		},
		Search:   strings.TrimSpace(in.Search),
		Status:   status,
		DateFrom: dateFrom,
		DateTo:   dateTo,
	}, nil
}
