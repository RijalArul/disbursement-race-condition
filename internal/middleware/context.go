package middleware

// Gin context keys shared between middleware layers. Auth middleware (AUTH-04)
// populates the identity keys; AccessLog and handlers read them. Keeping them
// as constants stops the same string literal from being retyped — and silently
// mistyped — at each call site.
const (
	ContextKeyRequestID = "request_id"
	ContextKeyUserID    = "user_id"
	ContextKeyUsername  = "username"
	ContextKeyRole      = "role"
)
