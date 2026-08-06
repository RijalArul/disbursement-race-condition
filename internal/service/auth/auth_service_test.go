package auth

import (
	"context"
	"testing"
	"time"

	authconst "github.com/RijalArul/disbursement-race-condition/internal/constants/auth"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/hash"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/jwt"
)

type fakeUserRepo struct {
	byUsername map[string]*models.User
	byID       map[string]*models.User
}

func (f *fakeUserRepo) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	if u, ok := f.byUsername[username]; ok {
		return u, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeUserRepo) FindByID(ctx context.Context, id string) (*models.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, domain.ErrNotFound
}

type fakeRefreshTokenRepo struct {
	byHash map[string]*models.RefreshToken
	byID   map[string]*models.RefreshToken
}

func newFakeRefreshTokenRepo() *fakeRefreshTokenRepo {
	return &fakeRefreshTokenRepo{byHash: map[string]*models.RefreshToken{}, byID: map[string]*models.RefreshToken{}}
}

func (f *fakeRefreshTokenRepo) Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (*models.RefreshToken, error) {
	rt := &models.RefreshToken{ID: tokenHash[:8], UserID: userID, TokenHash: tokenHash, ExpiresAt: expiresAt}
	f.byHash[tokenHash] = rt
	f.byID[rt.ID] = rt
	return rt, nil
}

func (f *fakeRefreshTokenRepo) FindByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	if rt, ok := f.byHash[tokenHash]; ok {
		return rt, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeRefreshTokenRepo) Revoke(ctx context.Context, id string) error {
	if rt, ok := f.byID[id]; ok {
		now := time.Now()
		rt.RevokedAt = &now
	}
	return nil
}

func (f *fakeRefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID string) error {
	now := time.Now()
	for _, rt := range f.byID {
		if rt.UserID == userID && rt.RevokedAt == nil {
			rt.RevokedAt = &now
		}
	}
	return nil
}

func newTestAuthService() (*Service, *fakeUserRepo, *fakeRefreshTokenRepo) {
	pwHash, _ := hash.HashPassword("correct-password")
	user := &models.User{ID: "user-1", Username: "operator1", PasswordHash: pwHash, Role: domain.RoleOperator}

	users := &fakeUserRepo{
		byUsername: map[string]*models.User{"operator1": user},
		byID:       map[string]*models.User{"user-1": user},
	}
	refreshTokens := newFakeRefreshTokenRepo()
	issuer := jwt.NewIssuer("test-secret", 15*time.Minute)

	return NewService(users, refreshTokens, issuer, 7*24*time.Hour), users, refreshTokens
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		password    string
		wantErrMsg  string
		wantSuccess bool
	}{
		{name: "correct credentials", username: "operator1", password: "correct-password", wantSuccess: true},
		{name: "unknown username", username: "ghost", password: "whatever", wantErrMsg: authconst.InvalidCredentials},
		{name: "wrong password", username: "operator1", password: "wrong-password", wantErrMsg: authconst.InvalidCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newTestAuthService()
			tokens, err := svc.Login(context.Background(), tt.username, tt.password)

			if tt.wantSuccess {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				if tokens.AccessToken == "" || tokens.RefreshToken == "" {
					t.Fatalf("expected non-empty tokens, got %+v", tokens)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error, got success")
			}
			if domain.ClientMessage(err) != tt.wantErrMsg {
				t.Fatalf("expected message %q, got %q", tt.wantErrMsg, domain.ClientMessage(err))
			}
		})
	}
}

func TestLogin_SameErrorForUnknownUserAndWrongPassword(t *testing.T) {
	svc, _, _ := newTestAuthService()

	_, err1 := svc.Login(context.Background(), "ghost", "whatever")
	_, err2 := svc.Login(context.Background(), "operator1", "wrong-password")

	msg1, msg2 := domain.ClientMessage(err1), domain.ClientMessage(err2)
	if msg1 != msg2 {
		t.Fatalf("expected identical messages to prevent user enumeration, got %q vs %q", msg1, msg2)
	}
}

func TestRefresh(t *testing.T) {
	t.Run("valid token rotates and returns new pair", func(t *testing.T) {
		svc, _, refreshTokens := newTestAuthService()
		login, err := svc.Login(context.Background(), "operator1", "correct-password")
		if err != nil {
			t.Fatalf("setup login failed: %v", err)
		}

		newTokens, err := svc.Refresh(context.Background(), login.RefreshToken)
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if newTokens.RefreshToken == login.RefreshToken {
			t.Fatalf("expected rotated refresh token to differ from original")
		}

		oldHash := hash.HashToken(login.RefreshToken)
		oldRT, _ := refreshTokens.FindByHash(context.Background(), oldHash)
		if oldRT.RevokedAt == nil {
			t.Fatalf("expected old refresh token to be revoked after rotation")
		}
	})

	t.Run("unknown token rejected", func(t *testing.T) {
		svc, _, _ := newTestAuthService()
		_, err := svc.Refresh(context.Background(), "not-a-real-token")
		if domain.ClientMessage(err) != authconst.InvalidRefreshToken {
			t.Fatalf("expected invalid refresh token error, got %v", err)
		}
	})

	t.Run("expired token rejected", func(t *testing.T) {
		svc, _, refreshTokens := newTestAuthService()
		raw, hashed, _ := hash.NewRefreshToken()
		refreshTokens.Create(context.Background(), "user-1", hashed, time.Now().Add(-time.Minute))

		_, err := svc.Refresh(context.Background(), raw)
		if domain.ClientMessage(err) != authconst.InvalidRefreshToken {
			t.Fatalf("expected invalid refresh token error, got %v", err)
		}
	})

	t.Run("reuse of revoked token revokes all tokens for user", func(t *testing.T) {
		svc, _, refreshTokens := newTestAuthService()
		login, _ := svc.Login(context.Background(), "operator1", "correct-password")

		// Legitimate rotation.
		rotated, err := svc.Refresh(context.Background(), login.RefreshToken)
		if err != nil {
			t.Fatalf("first refresh failed: %v", err)
		}

		// Attacker replays the now-revoked original token.
		_, err = svc.Refresh(context.Background(), login.RefreshToken)
		if domain.ClientMessage(err) != authconst.InvalidRefreshToken {
			t.Fatalf("expected reuse to be rejected, got %v", err)
		}

		// The legitimate rotated token must now also be revoked.
		rotatedHash := hash.HashToken(rotated.RefreshToken)
		rotatedRT, _ := refreshTokens.FindByHash(context.Background(), rotatedHash)
		if rotatedRT.RevokedAt == nil {
			t.Fatalf("expected reuse detection to revoke all tokens for the user, including the currently valid one")
		}
	})
}

func TestLogout(t *testing.T) {
	t.Run("valid token revoked, subsequent refresh rejected", func(t *testing.T) {
		svc, _, _ := newTestAuthService()
		login, _ := svc.Login(context.Background(), "operator1", "correct-password")

		if err := svc.Logout(context.Background(), login.RefreshToken); err != nil {
			t.Fatalf("expected logout to succeed, got %v", err)
		}

		_, err := svc.Refresh(context.Background(), login.RefreshToken)
		if domain.ClientMessage(err) != authconst.InvalidRefreshToken {
			t.Fatalf("expected refresh to be rejected after logout, got %v", err)
		}
	})

	t.Run("idempotent: logging out twice does not error", func(t *testing.T) {
		svc, _, _ := newTestAuthService()
		login, _ := svc.Login(context.Background(), "operator1", "correct-password")

		if err := svc.Logout(context.Background(), login.RefreshToken); err != nil {
			t.Fatalf("first logout failed: %v", err)
		}
		if err := svc.Logout(context.Background(), login.RefreshToken); err != nil {
			t.Fatalf("second logout should not error, got %v", err)
		}
	})

	t.Run("unknown token does not error", func(t *testing.T) {
		svc, _, _ := newTestAuthService()
		if err := svc.Logout(context.Background(), "never-issued"); err != nil {
			t.Fatalf("expected no error for unknown token, got %v", err)
		}
	})
}
