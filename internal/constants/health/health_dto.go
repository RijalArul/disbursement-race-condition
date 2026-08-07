package health

// Response is the body of a successful GET /health.
type Response struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}
