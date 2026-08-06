package middleware

import "context"

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

type identityCtxKey struct{}

// Identity is the authenticated caller, carried on context.Context so
// services read it there instead of trusting request body fields like
// created_by/approved_by.
type Identity struct {
	UserID   string
	Username string
	Role     string
}

func WithIdentity(ctx context.Context, userID, username, role string) context.Context {
	return context.WithValue(ctx, identityCtxKey{}, Identity{UserID: userID, Username: username, Role: role})
}

// IdentityFromCtx returns the authenticated caller, or ok=false if Auth
// middleware never ran on this request path.
func IdentityFromCtx(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityCtxKey{}).(Identity)
	return id, ok
}
