package disbursement

import (
	"time"

	"gorm.io/gorm"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
)

func filterBySearch(db *gorm.DB, search string) *gorm.DB {
	if search == "" {
		return db
	}
	return db.Where("recipient_name ILIKE ?", "%"+search+"%")
}

func filterByStatus(db *gorm.DB, status string) *gorm.DB {
	if status == "" {
		return db
	}
	return db.Where("status = ?", status)
}

func filterByDateFrom(db *gorm.DB, from *time.Time) *gorm.DB {
	if from == nil {
		return db
	}
	return db.Where("created_at >= ?", *from)
}

func filterByDateTo(db *gorm.DB, to *time.Time) *gorm.DB {
	if to == nil {
		return db
	}
	return db.Where("created_at <= ?", *to)
}

// orderClause re-validates sort_by/sort_order against the whitelist as a
// second line of defense — ORDER BY column/direction can't be bind
// parameters in SQL, so this repo must never trust a caller-supplied value.
//
// id is appended as a tie-breaker because none of the sortable columns is
// unique. Rows sharing an amount, a status, or a created_at have no defined
// order without it, and Postgres is free to return them differently per query
// — which makes LIMIT/OFFSET pagination repeat a row on one page and drop it
// from the next. id is the primary key and monotonic with insertion, so it
// settles ties deterministically.
func orderClause(sortBy, sortOrder string) string {
	if !domain.AllowedSortFields[sortBy] {
		sortBy = "created_at"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	return sortBy + " " + sortOrder + ", id " + sortOrder
}
