package audit

// Action values written to audit_logs. Only these 3 — reads/login/refresh stay out of the table.
const (
	ActionCreated       = "created"
	ActionStatusChanged = "status_changed"
	ActionDeleted       = "deleted"
)

const EntityTypeDisbursement = "disbursement"

// Validation messages for GET /audit-logs query params.
const (
	InvalidPage      = "page must be a positive integer"
	InvalidLimit     = "limit must be a positive integer"
	InvalidAction    = "action must be one of created, status_changed, deleted"
	InvalidDateFrom  = "date_from must be in YYYY-MM-DD format"
	InvalidDateTo    = "date_to must be in YYYY-MM-DD format"
	DateRangeInvalid = "date_from must not be after date_to"
	InvalidSortBy    = "sort_by must be one of created_at"
	InvalidSortOrder = "sort_order must be asc or desc"
)
