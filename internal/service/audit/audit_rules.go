package audit

import (
	"strings"
	"time"

	auditconst "github.com/RijalArul/disbursement-race-condition/internal/constants/audit"
	respconst "github.com/RijalArul/disbursement-race-condition/internal/constants/response"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/pagination"
)

const dateLayout = "2006-01-02"

func parseAction(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	switch raw {
	case auditconst.ActionCreated, auditconst.ActionStatusChanged, auditconst.ActionDeleted:
		return raw, nil
	default:
		return "", domain.Invalid(auditconst.InvalidAction)
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
	if !domain.AllowedAuditSortFields[raw] {
		return "", domain.Invalid(auditconst.InvalidSortBy)
	}
	return raw, nil
}

func parseSortOrder(raw string) (string, error) {
	if raw == "" {
		return "desc", nil
	}
	if raw != "asc" && raw != "desc" {
		return "", domain.Invalid(auditconst.InvalidSortOrder)
	}
	return raw, nil
}

// validateList parses and validates every GET /audit-logs query param,
// returning a typed ListQuery ready for the repository.
func validateList(in auditconst.ListRequest) (auditconst.ListQuery, error) {
	page, err := pagination.ParsePage(in.Page, auditconst.InvalidPage)
	if err != nil {
		return auditconst.ListQuery{}, err
	}
	limit, err := pagination.ParseLimit(in.Limit, auditconst.InvalidLimit)
	if err != nil {
		return auditconst.ListQuery{}, err
	}
	action, err := parseAction(in.Action)
	if err != nil {
		return auditconst.ListQuery{}, err
	}
	dateFrom, err := parseDate(in.DateFrom, auditconst.InvalidDateFrom)
	if err != nil {
		return auditconst.ListQuery{}, err
	}
	dateTo, err := parseDate(in.DateTo, auditconst.InvalidDateTo)
	if err != nil {
		return auditconst.ListQuery{}, err
	}
	if dateFrom != nil && dateTo != nil && dateFrom.After(*dateTo) {
		return auditconst.ListQuery{}, domain.Invalid(auditconst.DateRangeInvalid)
	}
	sortBy, err := parseSortBy(in.SortBy)
	if err != nil {
		return auditconst.ListQuery{}, err
	}
	sortOrder, err := parseSortOrder(in.SortOrder)
	if err != nil {
		return auditconst.ListQuery{}, err
	}

	return auditconst.ListQuery{
		PageQuery: respconst.PageQuery{
			Page:      page,
			Limit:     limit,
			SortBy:    sortBy,
			SortOrder: sortOrder,
		},
		EntityID: strings.TrimSpace(in.EntityID),
		Action:   action,
		DateFrom: dateFrom,
		DateTo:   dateTo,
	}, nil
}
