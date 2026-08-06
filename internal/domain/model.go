package domain

import "time"

type User struct {
	ID           string    `gorm:"column:id;primaryKey"`
	Username     string    `gorm:"column:username"`
	PasswordHash string    `gorm:"column:password_hash"`
	Role         UserRole  `gorm:"column:role"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (User) TableName() string { return "users" }

type RefreshToken struct {
	ID        string     `gorm:"column:id;primaryKey"`
	UserID    string     `gorm:"column:user_id"`
	TokenHash string     `gorm:"column:token_hash"`
	ExpiresAt time.Time  `gorm:"column:expires_at"`
	RevokedAt *time.Time `gorm:"column:revoked_at"`
	CreatedAt time.Time  `gorm:"column:created_at"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }

type Disbursement struct {
	ID            string             `gorm:"column:id;primaryKey"`
	RecipientName string             `gorm:"column:recipient_name"`
	AccountNumber string             `gorm:"column:account_number"`
	Amount        int64              `gorm:"column:amount"`
	AdminFee      int64              `gorm:"column:admin_fee"`
	Status        DisbursementStatus `gorm:"column:status"`
	CreatedBy     string             `gorm:"column:created_by"`
	ApprovedBy    *string            `gorm:"column:approved_by"`
	CreatedAt     time.Time          `gorm:"column:created_at"`
	UpdatedAt     time.Time          `gorm:"column:updated_at"`
	DeletedAt     *time.Time         `gorm:"column:deleted_at"`
}

func (Disbursement) TableName() string { return "disbursements" }

type IdempotencyKey struct {
	ID              string           `gorm:"column:id;primaryKey"`
	UserID          string           `gorm:"column:user_id"`
	IdempotencyKey  string           `gorm:"column:idempotency_key"`
	RequestHash     string           `gorm:"column:request_hash"`
	State           IdempotencyState `gorm:"column:state"`
	ResponseStatus  *int             `gorm:"column:response_status"`
	ResponseBody    []byte           `gorm:"column:response_body"`
	CreatedAt       time.Time        `gorm:"column:created_at"`
	ExpiresAt       time.Time        `gorm:"column:expires_at"`
}

func (IdempotencyKey) TableName() string { return "idempotency_keys" }

type AuditLog struct {
	ID         string    `gorm:"column:id;primaryKey"`
	ActorID    string    `gorm:"column:actor_id"`
	Action     string    `gorm:"column:action"`
	EntityType string    `gorm:"column:entity_type"`
	EntityID   string    `gorm:"column:entity_id"`
	Metadata   []byte    `gorm:"column:metadata"`
	RequestID  string    `gorm:"column:request_id"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }
