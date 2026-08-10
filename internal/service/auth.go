package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/dgrco/quikslate/internal/auth"
	"github.com/dgrco/quikslate/internal/domain"
)

// AuthService implements registration, login, refresh-token rotation, and
// logout, plus the authorization-context lookups the auth middleware calls
// on every request.

type AuthService struct {
	repo      domain.Repo
	jwtSecret string
}

// NewAuthService constructs an AuthService backed by repo, signing JWTs with jwtSecret.
func NewAuthService(repo domain.Repo, jwtSecret string) *AuthService {
	return &AuthService{
		repo:      repo,
		jwtSecret: jwtSecret,
	}
}

type AuthResult struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// Register creates a new user with a hashed password and issues an initial
// access/refresh token pair for them.
func (s *AuthService) Register(ctx context.Context, email, name, password string) (*AuthResult, error) {
	// Normalize email
	email = domain.NormalizeEmail(email)

	// validate email and password
	if err := domain.ValidateUserRegistrationCredentials(email, password); err != nil {
		return nil, err
	}

	// hash password
	hashed, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// create user
	user, err := s.repo.CreateUser(ctx, email, name, hashed)
	if err != nil {
		return nil, fmt.Errorf("failed to create user with business: %w", err)
	}

	return s.generateTokens(ctx, s.repo, user.Id)
}

// Login verifies email/password credentials and, on success, issues a new
// access/refresh token pair. Returns domain.ErrInvalidCredentials for both an
// unknown email and a wrong password, so callers can't distinguish the two.
func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	// Normalize email
	email = domain.NormalizeEmail(email)

	user, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("login error: %w", err)
	}

	if !auth.CheckPassword(password, user.Password) {
		return nil, domain.ErrInvalidCredentials
	}

	return s.generateTokens(ctx, s.repo, user.Id)
}

// Refresh rotates a refresh token: the presented token is looked up by its
// hash, checked for expiry, and deleted, and a brand-new access/refresh pair
// is issued in its place, all inside one transaction, so a crash mid-rotation
// can't leave the caller with neither a valid old nor new token.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*AuthResult, error) {
	hashedToken := hashToken(refreshToken)

	// look up the refresh token in the database
	stored, err := s.repo.GetRefreshToken(ctx, hashedToken)
	if err != nil {
		return nil, domain.ErrInvalidRefreshToken
	}

	// check it hasn't expired
	if time.Now().After(stored.ExpiresAt) {
		if err := s.repo.DeleteRefreshToken(ctx, hashedToken); err != nil {
			return nil, fmt.Errorf("failed to rotate refresh token: %w", err)
		}
		return nil, domain.ErrInvalidRefreshToken
	}

	// Wrap in transaction:
	tx, err := s.repo.BeginTransaction(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txRepo := s.repo.WithTx(tx)

	// rotate the token: delete old one, issue new one
	if err := txRepo.DeleteRefreshToken(ctx, hashedToken); err != nil {
		return nil, fmt.Errorf("failed to rotate refresh token: %w", err)
	}

	result, err := s.generateTokens(ctx, txRepo, stored.UserId)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

// Logout revokes a refresh token by deleting its stored hash, so it can no
// longer be used to mint new access tokens.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	hashedToken := hashToken(refreshToken)
	return s.repo.DeleteRefreshToken(ctx, hashedToken)
}

// generateTokens creates a JWT and a refresh token for a given user
func (s *AuthService) generateTokens(
	ctx context.Context,
	repo domain.Repo,
	userId string,
) (*AuthResult, error) {
	// generate JWT
	accessToken, err := auth.GenerateAccessToken(userId, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// generate a random refresh token
	refreshToken, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token")
	}

	// hash the refresh token for storage
	hashedToken := hashToken(refreshToken)

	// store hashed refresh token in database
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	_, err = repo.CreateRefreshToken(ctx, userId, hashedToken, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &AuthResult{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

// generateSecureToken generates n random bytes and hex encodes each,
// resulting in a string of length 2n.
func generateSecureToken(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// hashToken computes the SHA256 sum of a token and returns a hex-encoded string
// of length 64.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Authz

// GetBusinessMemberAuthzContext fetches the caller's admin/primary-admin
// status for businessId, fresh from the database. Called by
// RequireBusinessMember on every request so role/admin changes take effect
// immediately, instead of only after the caller's JWT expires.
func (s *AuthService) GetBusinessMemberAuthzContext(
	ctx context.Context,
	userId,
	businessId string,
) (domain.BusinessMemberAuthzContext, error) {
	ac, err := s.repo.GetBusinessMemberAuthzContext(ctx, userId, businessId)
	if err != nil {
		return domain.BusinessMemberAuthzContext{}, fmt.Errorf("failed to get business member authorization context: %w", err)
	}
	return ac, nil
}

// GetLocationMemberAuthzContext fetches the caller's admin/primary-admin
// status plus their role at locationId, fresh from the database. Called by
// RequireLocationMember on every request; the underlying query also verifies
// locationId actually belongs to businessId, failing the request otherwise.
func (s *AuthService) GetLocationMemberAuthzContext(
	ctx context.Context,
	userId,
	businessId,
	locationId string,
) (domain.LocationMemberAuthzContext, error) {
	ac, err := s.repo.GetLocationMemberAuthzContext(ctx, userId, businessId, locationId)
	if err != nil {
		return domain.LocationMemberAuthzContext{}, fmt.Errorf("failed to get location member authorization context: %w", err)
	}
	return ac, nil
}
