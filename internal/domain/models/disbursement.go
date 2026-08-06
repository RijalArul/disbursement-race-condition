package models

import (
	"time"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
)

type Disbursement struct {
	ID            string  `gorm:"column:id;primaryKey"`
	RecipientName string  `gorm:"column:recipient_name"`
	AccountNumber string  `gorm:"column:account_number"`
	BankCode      string  `gorm:"column:bank_code"`
	Note          *string `gorm:"column:note"`
	// Amount and AdminFee are int64 end to end and BIGINT in Postgres. Money
	// never touches a float type anywhere on this path.
	Amount     int64                     `gorm:"column:amount"`
	AdminFee   int64                     `gorm:"column:admin_fee"`
	Status     domain.DisbursementStatus `gorm:"column:status"`
	CreatedBy  string                    `gorm:"column:created_by"`
	ApprovedBy *string                   `gorm:"column:approved_by"`
	CreatedAt  time.Time                 `gorm:"column:created_at"`
	UpdatedAt  time.Time                 `gorm:"column:updated_at"`
	DeletedAt  *time.Time                `gorm:"column:deleted_at"`
}

func (Disbursement) TableName() string { return "disbursements" }
