package audit

import (
	"context"

	"gorm.io/gorm"

	auditconst "github.com/RijalArul/disbursement-race-condition/internal/constants/audit"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
)

// AuditRepository is consumed by the service as an interface so unit tests
// can fake it and run without a live database.
type AuditRepository interface {
	Create(ctx context.Context, a *models.AuditLog) error
	List(ctx context.Context, q auditconst.ListQuery) ([]models.AuditLog, int64, error)
}

type auditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepository{db: db}
}

// Create omits id/created_at so the audit_log_id_seq default fills id.
func (r *auditRepository) Create(ctx context.Context, a *models.AuditLog) error {
	return r.db.WithContext(ctx).
		Omit("ID", "CreatedAt").
		Create(a).Error
}

func (r *auditRepository) List(ctx context.Context, q auditconst.ListQuery) ([]models.AuditLog, int64, error) {
	base := r.db.WithContext(ctx).Model(&models.AuditLog{})
	base = filterByEntityID(base, q.EntityID)
	base = filterByAction(base, q.Action)
	base = filterByAuditDateFrom(base, q.DateFrom)
	base = filterByAuditDateTo(base, q.DateTo)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []models.AuditLog
	err := base.
		Order(auditOrderClause(q.SortBy, q.SortOrder)).
		Limit(q.Limit).
		Offset((q.Page - 1) * q.Limit).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}
