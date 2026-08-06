package models

import "time"

type AuditLog struct {
	ID         string    `gorm:"column:id;primaryKey"`
	ActorID    string    `gorm:"column:actor_id"`
	Action     string    `gorm:"column:action"`
	EntityType string    `gorm:"column:entity_type"`
	EntityID   string    `gorm:"column:entity_id"`
	Before     []byte    `gorm:"column:before"`
	After      []byte    `gorm:"column:after"`
	RequestID  string    `gorm:"column:request_id"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }
