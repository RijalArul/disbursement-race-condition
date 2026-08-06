package disbursement

import "time"

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
