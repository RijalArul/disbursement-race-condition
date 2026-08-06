package response

// PageQuery is the parsed, validated page/limit/sort a list endpoint reads
// from query params. Reused across modules with pagination.
type PageQuery struct {
	Page      int
	Limit     int
	SortBy    string
	SortOrder string
}

// PageMeta is the pagination block returned alongside a list.
type PageMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}
