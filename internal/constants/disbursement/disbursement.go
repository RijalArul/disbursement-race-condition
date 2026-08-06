package disbursement

// Business thresholds for disbursements (DSB-01). Kept as named constants so
// the boundary values appear once and can be referenced by the tests that
// assert on them, rather than being retyped as literals at each site.
const (
	// MinAmount is the smallest amount that may be disbursed.
	// Mirrored by CHECK (amount >= 10000) in migration 000001.
	MinAmount int64 = 10_000

	// AdminFeeThreshold is the amount at and above which the higher admin fee
	// applies. The comparison is >=, so an amount of exactly this value is
	// charged AdminFeeHigh.
	AdminFeeThreshold int64 = 5_000_000

	AdminFeeLow  int64 = 2_500
	AdminFeeHigh int64 = 5_000
)

// Validation messages for POST /disbursements. Each names the offending field
// so a 400 tells the caller what to fix.
const (
	InvalidBody           = "invalid request body"
	RecipientNameRequired = "recipient_name is required"
	AccountNumberRequired = "account_number is required"
	BankCodeRequired      = "bank_code is required"
	AmountBelowMinimum    = "amount must be at least 10000"
)

// Unauthenticated is returned when the service cannot read an identity from
// context. It signals that the route was wired without the auth middleware,
// since created_by is taken from JWT claims and never from the body.
const Unauthenticated = "authentication required"
