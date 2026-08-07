package audit

import (
	"time"

	"gorm.io/gorm"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
)

func filterByEntityID(db *gorm.DB, entityID string) *gorm.DB {
	if entityID == "" {
		return db
	}
	return db.Where("entity_id = ?", entityID)
}

func filterByAction(db *gorm.DB, action string) *gorm.DB {
	if action == "" {
		return db
	}
	return db.Where("action = ?", action)
}

func filterByAuditDateFrom(db *gorm.DB, from *time.Time) *gorm.DB {
	if from == nil {
		return db
	}
	return db.Where("created_at >= ?", *from)
}

func filterByAuditDateTo(db *gorm.DB, to *time.Time) *gorm.DB {
	if to == nil {
		return db
	}
	return db.Where("created_at <= ?", *to)
}

// auditOrderClause re-validates sort_by/sort_order against the whitelist as a
// second line of defense — ORDER BY column/direction can't be bind
// parameters in SQL, so this repo must never trust a caller-supplied value.
//
// id breaks ties on created_at for the same reason as in the disbursement
// repository: audit rows are written by a pool of workers, so several can land
// on the same timestamp, and without a unique tie-breaker paginated reads can
// repeat or skip them.
func auditOrderClause(sortBy, sortOrder string) string {
	if !domain.AllowedAuditSortFields[sortBy] {
		sortBy = "created_at"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	return sortBy + " " + sortOrder + ", id " + sortOrder
}
