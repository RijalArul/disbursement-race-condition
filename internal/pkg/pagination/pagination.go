package pagination

import (
	"strconv"

	respconst "github.com/RijalArul/disbursement-race-condition/internal/constants/response"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
)

// ParsePage parses the page query param, defaulting to respconst.DefaultPage
// when empty. invalidMsg is the caller's module-specific error message.
func ParsePage(raw, invalidMsg string) (int, error) {
	if raw == "" {
		return respconst.DefaultPage, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, domain.Invalid(invalidMsg)
	}
	return n, nil
}

// ParseLimit parses the limit query param, defaulting to respconst.DefaultLimit
// when empty and capping at respconst.MaxLimit.
func ParseLimit(raw, invalidMsg string) (int, error) {
	if raw == "" {
		return respconst.DefaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, domain.Invalid(invalidMsg)
	}
	if n > respconst.MaxLimit {
		n = respconst.MaxLimit
	}
	return n, nil
}

// BuildPageMeta computes total_pages, rounding up so a partial last page still counts.
func BuildPageMeta(page, limit int, total int64) respconst.PageMeta {
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	return respconst.PageMeta{Page: page, Limit: limit, Total: total, TotalPages: totalPages}
}
