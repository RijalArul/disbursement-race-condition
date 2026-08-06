package domain

type UserRole string

const (
	RoleSuperAdmin UserRole = "superadmin"
	RoleAdmin      UserRole = "admin"
	RoleOperator   UserRole = "operator"
)

type DisbursementStatus string

const (
	StatusPending  DisbursementStatus = "PENDING"
	StatusApproved DisbursementStatus = "APPROVED"
	StatusRejected DisbursementStatus = "REJECTED"
	StatusFailed   DisbursementStatus = "FAILED"
)

type IdempotencyState string

const (
	IdemStateProcessing IdempotencyState = "PROCESSING"
	IdemStateCompleted  IdempotencyState = "COMPLETED"
)

// AllowedSortFields whitelists columns usable in ORDER BY for GET /disbursements.
// sort_by must be validated against this before touching the query.
var AllowedSortFields = map[string]bool{
	"created_at": true,
	"amount":     true,
	"status":     true,
}

// AllowedAuditSortFields whitelists columns usable in ORDER BY for GET /audit-logs.
var AllowedAuditSortFields = map[string]bool{
	"created_at": true,
}
