package disbursement

import (
	"time"

	respconst "github.com/RijalArul/disbursement-race-condition/internal/constants/response"
)

// CreateRequest has no status/admin_fee/created_by/approved_by/id field — those are server-decided, never accepted from the body.
type CreateRequest struct {
	RecipientName string  `json:"recipient_name"`
	AccountNumber string  `json:"account_number"`
	BankCode      string  `json:"bank_code"`
	Amount        int64   `json:"amount"`
	Note          *string `json:"note"`
}

// CreateInput is the service's own contract, kept separate from CreateRequest (the wire-bound DTO) so it survives transport changes.
type CreateInput struct {
	RecipientName string
	AccountNumber string
	BankCode      string
	Amount        int64
	Note          *string
}

// UpdateStatusRequest has no approved_by field — that comes from JWT context, never the body.
type UpdateStatusRequest struct {
	Status string  `json:"status"`
	Note   *string `json:"note"`
}

// UpdateStatusInput is the service's own contract for PATCH /disbursements/:id/status.
type UpdateStatusInput struct {
	ID     string
	Status string
	Note   *string
}

// ListRequest is the raw, unvalidated GET /disbursements query params as
// bound from the URL — every field stays a string, since page/limit/dates
// need service-layer validation before they become typed values.
type ListRequest struct {
	Page      string
	Limit     string
	Search    string
	Status    string
	DateFrom  string
	DateTo    string
	SortBy    string
	SortOrder string
}

// ListQuery is the service's parsed, validated form of GET /disbursements
// query params.
type ListQuery struct {
	respconst.PageQuery
	Search   string
	Status   string
	DateFrom *time.Time
	DateTo   *time.Time
}

// Response is the client-facing shape of a disbursement. Note is a pointer so
// an absent note serialises as null rather than an empty string.
type Response struct {
	ID            string    `json:"id"`
	RecipientName string    `json:"recipient_name"`
	AccountNumber string    `json:"account_number"`
	BankCode      string    `json:"bank_code"`
	Note          *string   `json:"note"`
	Amount        int64     `json:"amount"`
	AdminFee      int64     `json:"admin_fee"`
	Status        string    `json:"status"`
	CreatedBy     string    `json:"created_by"`
	ApprovedBy    *string   `json:"approved_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
