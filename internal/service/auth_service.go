package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/RijalArul/disbursement-race-condition/internal/constants"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/hash"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/jwt"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/logger"
	"github.com/RijalArul/disbursement-race-condition/internal/repository"
)

type AuthTokens struct {
	AccessToken  string
	RefreshToken string
}

type AuthService struct {
	users         repository.UserRepository
	refreshTokens repository.RefreshTokenRepository
	issuer        *jwt.Issuer
	refreshTTL    time.Duration
}

func NewAuthService(users repository.UserRepository, refreshTokens repository.RefreshTokenRepository, issuer *jwt.Issuer, refreshTTL time.Duration) *AuthService {
	return &AuthService{users: users, refreshTokens: refreshTokens, issuer: issuer, refreshTTL: refreshTTL}
}

// Login verifies credentials and issues a fresh token pair. Every failure —
// unknown username or wrong password — returns the same generic error so the
// response can't be used to enumerate valid usernames.
func (s *AuthService) Login(ctx context.Context, username, password string) (*AuthTokens, error) {
	log := logger.FromCtx(ctx)

	user, err := s.users.FindByUsername(ctx, username)
	if err != nil {
		if err == domain.ErrNotFound {
			log.Warn("login failed", slog.String("reason", "user_not_found"))
			return nil, domain.Unauthorized(constants.AuthInvalidCredentials)
		}
		return nil, fmt.Errorf("login lookup user: %w", err)
	}

	if !hash.ComparePassword(user.PasswordHash, password) {
		log.Warn("login failed", slog.String("reason", "bad_password"), slog.String("user_id", user.ID))
		return nil, domain.Unauthorized(constants.AuthInvalidCredentials)
	}

	return s.issueTokenPair(ctx, user.ID, user.Username, user.Role)
}

// Refresh exchanges a valid refresh token for a new access token, rotating
// the refresh token in the process. Reuse of an already-revoked token
// revokes every token belonging to that user — it is the signal that the
// token was stolen and reused after the legitimate client rotated past it.
func (s *AuthService) Refresh(ctx context.Context, rawToken string) (*AuthTokens, error) {
	log := logger.FromCtx(ctx)
	tokenHash := hash.HashToken(rawToken)

	stored, err := s.refreshTokens.FindByHash(ctx, tokenHash)
	if err != nil {
		if err == domain.ErrNotFound {
			return nil, domain.Unauthorized(constants.AuthInvalidRefreshToken)
		}
		return nil, fmt.Errorf("refresh lookup token: %w", err)
	}

	if stored.RevokedAt != nil {
		log.Warn("revoked refresh token reused, revoking all tokens for user", slog.String("user_id", stored.UserID))
		if err := s.refreshTokens.RevokeAllForUser(ctx, stored.UserID); err != nil {
			return nil, fmt.Errorf("revoke all tokens after reuse detection: %w", err)
		}
		return nil, domain.Unauthorized(constants.AuthInvalidRefreshToken)
	}

	if stored.ExpiresAt.Before(time.Now()) {
		return nil, domain.Unauthorized(constants.AuthInvalidRefreshToken)
	}

	user, err := s.userByID(ctx, stored.UserID)
	if err != nil {
		return nil, err
	}

	if err := s.refreshTokens.Revoke(ctx, stored.ID); err != nil {
		return nil, fmt.Errorf("revoke rotated token: %w", err)
	}

	return s.issueTokenPair(ctx, user.ID, user.Username, user.Role)
}

// Logout revokes the refresh token if it exists and is still active.
// Revoking an already-revoked or unknown token is not an error — logout is
// idempotent by requirement.
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	tokenHash := hash.HashToken(rawToken)

	stored, err := s.refreshTokens.FindByHash(ctx, tokenHash)
	if err != nil {
		if err == domain.ErrNotFound {
			return nil
		}
		return fmt.Errorf("logout lookup token: %w", err)
	}

	if stored.RevokedAt != nil {
		return nil
	}

	if err := s.refreshTokens.Revoke(ctx, stored.ID); err != nil {
		return fmt.Errorf("logout revoke token: %w", err)
	}
	return nil
}

func (s *AuthService) issueTokenPair(ctx context.Context, userID, username string, role domain.UserRole) (*AuthTokens, error) {
	access, err := s.issuer.Generate(userID, username, role)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	rawRefresh, hashedRefresh, err := hash.NewRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	if _, err := s.refreshTokens.Create(ctx, userID, hashedRefresh, time.Now().Add(s.refreshTTL)); err != nil {
		return nil, fmt.Errorf("persist refresh token: %w", err)
	}

	return &AuthTokens{AccessToken: access, RefreshToken: rawRefresh}, nil
}

func (s *AuthService) userByID(ctx context.Context, userID string) (*domain.User, error) {
	return s.users.FindByID(ctx, userID)
}
