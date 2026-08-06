package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/jwt"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter(issuer *jwt.Issuer, allowed ...domain.UserRole) *gin.Engine {
	r := gin.New()
	group := r.Group("/protected")
	group.Use(Auth(issuer))
	if len(allowed) > 0 {
		group.Use(RequireRole(allowed...))
	}
	group.GET("/resource", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func doRequest(r *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected/resource", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuth(t *testing.T) {
	issuer := jwt.NewIssuer("test-secret", 15*time.Minute)
	validToken, _ := issuer.Generate("user-1", "operator1", domain.RoleOperator)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{name: "missing header", authHeader: "", wantStatus: http.StatusUnauthorized},
		{name: "malformed header no bearer prefix", authHeader: validToken, wantStatus: http.StatusUnauthorized},
		{name: "empty bearer value", authHeader: "Bearer ", wantStatus: http.StatusUnauthorized},
		{name: "garbage token", authHeader: "Bearer not-a-jwt", wantStatus: http.StatusUnauthorized},
		{name: "wrong signature", authHeader: "Bearer " + tamperSignature(validToken), wantStatus: http.StatusUnauthorized},
		{name: "valid token", authHeader: "Bearer " + validToken, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupRouter(issuer)
			w := doRequest(r, tt.authHeader)
			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d (body: %s)", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	issuer := jwt.NewIssuer("test-secret", -time.Minute) // already expired
	expiredToken, _ := issuer.Generate("user-1", "operator1", domain.RoleOperator)

	r := setupRouter(issuer)
	w := doRequest(r, "Bearer "+expiredToken)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", w.Code)
	}
}

func TestRequireRole(t *testing.T) {
	issuer := jwt.NewIssuer("test-secret", 15*time.Minute)

	tests := []struct {
		name       string
		tokenRole  domain.UserRole
		allowed    []domain.UserRole
		wantStatus int
	}{
		{name: "operator allowed on operator route", tokenRole: domain.RoleOperator, allowed: []domain.UserRole{domain.RoleOperator, domain.RoleAdmin, domain.RoleSuperAdmin}, wantStatus: http.StatusOK},
		{name: "operator rejected on admin-only route", tokenRole: domain.RoleOperator, allowed: []domain.UserRole{domain.RoleAdmin, domain.RoleSuperAdmin}, wantStatus: http.StatusForbidden},
		{name: "admin allowed on admin route", tokenRole: domain.RoleAdmin, allowed: []domain.UserRole{domain.RoleAdmin, domain.RoleSuperAdmin}, wantStatus: http.StatusOK},
		{name: "admin rejected on superadmin-only route", tokenRole: domain.RoleAdmin, allowed: []domain.UserRole{domain.RoleSuperAdmin}, wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, _ := issuer.Generate("user-1", "someuser", tt.tokenRole)
			r := setupRouter(issuer, tt.allowed...)
			w := doRequest(r, "Bearer "+token)
			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d (body: %s)", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestRequireRole_ReturnsForbiddenNotUnauthorized(t *testing.T) {
	issuer := jwt.NewIssuer("test-secret", 15*time.Minute)
	token, _ := issuer.Generate("user-1", "operator1", domain.RoleOperator)

	r := setupRouter(issuer, domain.RoleSuperAdmin)
	w := doRequest(r, "Bearer "+token)

	if w.Code != http.StatusForbidden {
		t.Fatalf("insufficient role must be 403, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
}

// tamperSignature flips the last character of a JWT's signature segment so
// verification fails without needing a second signing key.
func tamperSignature(token string) string {
	if len(token) == 0 {
		return token
	}
	last := token[len(token)-1]
	replacement := byte('A')
	if last == 'A' {
		replacement = 'B'
	}
	return token[:len(token)-1] + string(replacement)
}
