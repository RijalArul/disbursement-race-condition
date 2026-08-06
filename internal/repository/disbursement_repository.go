package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
)

// DisbursementRepository is consumed by the service as an interface so unit
// tests can fake it and run without a live database.
type DisbursementRepository interface {
	Create(ctx context.Context, d *models.Disbursement) error
}

type disbursementRepository struct {
	db *gorm.DB
}

func NewDisbursementRepository(db *gorm.DB) DisbursementRepository {
	return &disbursementRepository{db: db}
}

// Create omits id/created_at/updated_at so Postgres defaults (the DSB-000001
// sequence, etc.) fill them, read back via GORM's RETURNING.
func (r *disbursementRepository) Create(ctx context.Context, d *models.Disbursement) error {
	return r.db.WithContext(ctx).
		Omit("ID", "CreatedAt", "UpdatedAt").
		Create(d).Error
}
