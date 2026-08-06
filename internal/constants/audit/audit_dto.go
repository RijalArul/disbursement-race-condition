package audit

import (
	"time"

	respconst "github.com/RijalArul/disbursement-race-condition/internal/constants/response"
)

// ListRequest is the raw, unvalidated GET /audit-logs query params as bound
// from the URL — every field stays a string until service-layer validation.
type ListRequest struct {
	Page      string
	Limit     string
	EntityID  string
	Action    string
	DateFrom  string
	DateTo    string
	SortBy    string
	SortOrder string
}

// ListQuery is the service's parsed, validated form of GET /audit-logs query params.
type ListQuery struct {
	respconst.PageQuery
	EntityID string
	Action   string
	DateFrom *time.Time
	DateTo   *time.Time
}

// Response is the client-facing shape of an audit log entry.
type Response struct {
	ID        string      `json:"id"`
	EntityID  string      `json:"entity_id"`
	Action    string      `json:"action"`
	Actor     string      `json:"actor"`
	Before    interface{} `json:"before"`
	After     interface{} `json:"after"`
	CreatedAt time.Time   `json:"created_at"`
}
