package disbursement

// Business thresholds for disbursements (DSB-01).
const (
	// MinAmount mirrors CHECK (amount >= 10000) in migration 000001.
	MinAmount int64 = 10_000

	// AdminFeeThreshold: >= this amount charges AdminFeeHigh.
	AdminFeeThreshold int64 = 5_000_000

	AdminFeeLow  int64 = 2_500
	AdminFeeHigh int64 = 5_000
)

// Validation messages for POST /disbursements.
const (
	InvalidBody           = "invalid request body"
	RecipientNameRequired = "recipient_name is required"
	AccountNumberRequired = "account_number is required"
	BankCodeRequired      = "bank_code is required"
	AmountBelowMinimum    = "amount must be at least 10000"
)

// Unauthenticated means the service found no identity in context.
const Unauthenticated = "authentication required"

// NotFound covers both a missing id and a soft-deleted row.
const NotFound = "disbursement not found"

// Validation messages for PATCH /disbursements/:id/status.
const (
	InvalidStatusTransition = "status must be APPROVED or REJECTED"
	AlreadyDecided          = "disbursement already %s, status cannot be changed again"
)

// NotPending is returned when a non-PENDING disbursement is targeted by an
// operation that requires PENDING (delete, status change).
const NotPending = "only a PENDING disbursement can be %s"

// Validation messages for GET /disbursements query params.
const (
	InvalidPage      = "page must be a positive integer"
	InvalidLimit     = "limit must be a positive integer"
	InvalidStatus    = "status must be one of PENDING, APPROVED, REJECTED, FAILED"
	InvalidDateFrom  = "date_from must be in YYYY-MM-DD format"
	InvalidDateTo    = "date_to must be in YYYY-MM-DD format"
	DateRangeInvalid = "date_from must not be after date_to"
	InvalidSortBy    = "sort_by must be one of created_at, amount, status"
	InvalidSortOrder = "sort_order must be asc or desc"
)
