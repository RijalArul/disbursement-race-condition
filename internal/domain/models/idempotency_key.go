package models

import (
	"time"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
)

type IdempotencyKey struct {
	ID             string                  `gorm:"column:id;primaryKey"`
	UserID         string                  `gorm:"column:user_id"`
	IdempotencyKey string                  `gorm:"column:idempotency_key"`
	RequestHash    string                  `gorm:"column:request_hash"`
	State          domain.IdempotencyState `gorm:"column:state"`
	ResponseStatus *int                    `gorm:"column:response_status"`
	ResponseBody   []byte                  `gorm:"column:response_body"`
	CreatedAt      time.Time               `gorm:"column:created_at"`
	ExpiresAt      time.Time               `gorm:"column:expires_at"`
}

func (IdempotencyKey) TableName() string { return "idempotency_keys" }
