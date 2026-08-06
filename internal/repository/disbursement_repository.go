package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	disbconst "github.com/RijalArul/disbursement-race-condition/internal/constants/disbursement"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
)

// DisbursementRepository is consumed by the service as an interface so unit
// tests can fake it and run without a live database.
type DisbursementRepository interface {
	Create(ctx context.Context, d *models.Disbursement) error
	GetByID(ctx context.Context, id string) (*models.Disbursement, error)
	List(ctx context.Context, q disbconst.ListQuery) ([]models.Disbursement, int64, error)
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

func (r *disbursementRepository) GetByID(ctx context.Context, id string) (*models.Disbursement, error) {
	var d models.Disbursement
	err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		First(&d, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *disbursementRepository) List(ctx context.Context, q disbconst.ListQuery) ([]models.Disbursement, int64, error) {
	base := r.db.WithContext(ctx).Model(&models.Disbursement{}).Where("deleted_at IS NULL")
	base = filterBySearch(base, q.Search)
	base = filterByStatus(base, q.Status)
	base = filterByDateFrom(base, q.DateFrom)
	base = filterByDateTo(base, q.DateTo)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []models.Disbursement
	err := base.
		Order(orderClause(q.SortBy, q.SortOrder)).
		Limit(q.Limit).
		Offset((q.Page - 1) * q.Limit).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}
