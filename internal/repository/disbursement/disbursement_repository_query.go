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
func orderClause(sortBy, sortOrder string) string {
	if !domain.AllowedSortFields[sortBy] {
		sortBy = "created_at"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	return sortBy + " " + sortOrder
}
