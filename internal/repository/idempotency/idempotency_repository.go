package idempotency

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
)

// idempotencyTTL mirrors the DB column default (expires_at TIMESTAMPTZ ...
// DEFAULT (now() + interval '24 hours')) so a reservation is never left at
// its Go zero value, which would make every row look expired on arrival.
const idempotencyTTL = 24 * time.Hour

// IdempotencyRepository is consumed by the idempotency middleware as an interface so unit tests can fake it.
type IdempotencyRepository interface {
	TryReserve(ctx context.Context, userID, key, requestHash string) (reserved bool, err error)
	Find(ctx context.Context, userID, key string) (*models.IdempotencyKey, error)
	Complete(ctx context.Context, userID, key string, status int, body []byte) error
	Delete(ctx context.Context, userID, key string) error
}

type idempotencyRepository struct {
	db *gorm.DB
}

func NewIdempotencyRepository(db *gorm.DB) IdempotencyRepository {
	return &idempotencyRepository{db: db}
}

// TryReserve claims (userID, key) atomically. An expired row is overwritten as if it never
// existed; a live one blocks the claim, so reserved=false means a live row already exists.
func (r *idempotencyRepository) TryReserve(ctx context.Context, userID, key, requestHash string) (bool, error) {
	row := &models.IdempotencyKey{
		UserID:         userID,
		IdempotencyKey: key,
		RequestHash:    requestHash,
		State:          domain.IdemStateProcessing,
		ExpiresAt:      time.Now().Add(idempotencyTTL),
	}

	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "idempotency_key"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"request_hash", "state", "response_status", "response_body", "created_at", "expires_at",
			}),
			Where: clause.Where{Exprs: []clause.Expression{
				gorm.Expr("idempotency_keys.expires_at < now()"),
			}},
		}).
		Omit("ID").
		Create(row)
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected == 1, nil
}

func (r *idempotencyRepository) Find(ctx context.Context, userID, key string) (*models.IdempotencyKey, error) {
	var row models.IdempotencyKey
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND idempotency_key = ?", userID, key).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NotFound("idempotency key not found")
		}
		return nil, err
	}
	return &row, nil
}

func (r *idempotencyRepository) Complete(ctx context.Context, userID, key string, status int, body []byte) error {
	return r.db.WithContext(ctx).
		Model(&models.IdempotencyKey{}).
		Where("user_id = ? AND idempotency_key = ?", userID, key).
		Updates(map[string]interface{}{
			"state":           domain.IdemStateCompleted,
			"response_status": status,
			"response_body":   body,
		}).Error
}

func (r *idempotencyRepository) Delete(ctx context.Context, userID, key string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND idempotency_key = ?", userID, key).
		Delete(&models.IdempotencyKey{}).Error
}
