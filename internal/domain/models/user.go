package models

import (
	"time"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
)

type User struct {
	ID           string          `gorm:"column:id;primaryKey"`
	Username     string          `gorm:"column:username"`
	PasswordHash string          `gorm:"column:password_hash"`
	Role         domain.UserRole `gorm:"column:role"`
	CreatedAt    time.Time       `gorm:"column:created_at"`
}

func (User) TableName() string { return "users" }
