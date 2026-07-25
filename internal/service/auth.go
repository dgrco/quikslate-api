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
	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

type AuthService struct {
	repo      domain.Repo
	jwtSecret string
}

func NewAuthService(repo domain.Repo, jwtSecret string) *AuthService {
	return &AuthService{
		repo:      repo,
		jwtSecret: jwtSecret,
	}
}

type AuthResult struct {
	AccessToken               string                `json:"access_token,omitempty"`
	RefreshToken              string                `json:"refresh_token,omitempty"`
	RequiresBusinessSelection bool                  `json:"requires_business_selection"`
	Businesses                []domain.Business     `json:"businesses,omitempty"`
	RequiresLocationSelection bool                  `json:"requires_location_selection"`
	Locations                 []domain.LocationRole `json:"locations,omitempty"`
}

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

	// generate identity-only tokens (no business or location ids)
	return s.generateTokens(ctx, s.repo, user.Id, "", "", false, false, domain.EmptyRole)
}

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

	bms, err := s.repo.GetBusinessMembersByUserId(ctx, user.Id)
	if err != nil {
		return nil, fmt.Errorf("login error: %w", err)
	}

	businesses := []domain.Business{}
	for _, bm := range bms {
		b, err := s.repo.GetBusinessById(ctx, bm.BusinessId)
		if err != nil {
			return nil, fmt.Errorf("login error: %w", err)
		}
		businesses = append(businesses, b)
	}

	if len(businesses) == 0 {
		// Identity token
		return s.generateTokens(ctx, s.repo, user.Id, "", "", false, false, domain.EmptyRole)
	}

	if len(businesses) == 1 {
		// since this calls SelectBusiness and we aren't going through SelectionAuthMiddleware
		// we need to fill the context state here.
		ctx = context.WithValue(ctx, ctxkeys.UserId, user.Id)
		return s.SelectBusiness(ctx, businesses[0].Id)
	}

	accessToken, err := s.generateInterimAccessToken(user.Id)
	if err != nil {
		return nil, fmt.Errorf("login error: %w", err)
	}

	return &AuthResult{
		AccessToken:               accessToken,
		Businesses:                businesses,
		RequiresBusinessSelection: true,
	}, nil
}

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

	// fetch business member data for the user
	bm, err := s.repo.GetBusinessMember(ctx, stored.UserId, stored.BusinessId)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	locationId := stored.LocationId
	var role domain.LRole
	if stored.LocationId != "" && !bm.IsAdmin {
		lr, err := s.repo.GetLocationRole(ctx, stored.UserId, stored.LocationId, stored.BusinessId)
		if err != nil {
			return nil, domain.ErrNotFound
		}
		role = lr.Role
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

	result, err := s.generateTokens(ctx, txRepo, bm.UserId, bm.BusinessId, locationId, bm.IsPrimaryAdmin, bm.IsAdmin, role)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	hashedToken := hashToken(refreshToken)
	return s.repo.DeleteRefreshToken(ctx, hashedToken)
}

func (s *AuthService) SelectBusiness(ctx context.Context, businessId string) (*AuthResult, error) {
	userId := ctxkeys.GetUserId(ctx)

	bm, err := s.repo.GetBusinessMember(ctx, userId, businessId)
	if err != nil {
		return nil, fmt.Errorf("failed to select business: %w", err)
	}

	lrs, err := s.repo.GetLocationRolesByUserAndBusiness(ctx, userId, businessId)
	if err != nil {
		return nil, fmt.Errorf("failed to select business: %w", err)
	}

	if bm.IsAdmin && len(lrs) == 0 {
		return s.generateTokens(ctx, s.repo, userId, businessId, domain.EmptyLocation, bm.IsPrimaryAdmin, bm.IsAdmin, domain.EmptyRole)
	} else if len(lrs) == 1 {
		return s.generateTokens(ctx, s.repo, userId, businessId, lrs[0].LocationId, bm.IsPrimaryAdmin, bm.IsAdmin, lrs[0].Role)
	} else {
		// Many locations -> requires selection
		accessToken, err := s.generateInterimAccessToken(userId)
		if err != nil {
			return nil, fmt.Errorf("select business error: %w", err)
		}
		return &AuthResult{AccessToken: accessToken, Locations: lrs, RequiresLocationSelection: true}, nil
	}
}

func (s *AuthService) SelectLocation(ctx context.Context, businessId, locationId string) (*AuthResult, error) {
	userId := ctxkeys.GetUserId(ctx)

	// verify the requested user belongs to the requested business
	bm, err := s.repo.GetBusinessMember(ctx, userId, businessId)
	if err != nil {
		return nil, fmt.Errorf("failed to select location: %w", err)
	}

	// verify the requested location belongs to the requested business
	location, err := s.repo.GetLocationById(ctx, locationId)
	if err != nil {
		return nil, fmt.Errorf("failed to select location: %w", err)
	}
	if location.BusinessId != businessId {
		return nil, domain.ErrForbidden
	}

	role := domain.EmptyRole
	if !bm.IsAdmin {
		lr, err := s.repo.GetLocationRole(ctx, userId, locationId, businessId)
		if err != nil {
			return nil, fmt.Errorf("failed to select location: %w", err)
		}
		role = lr.Role
	}
	return s.generateTokens(ctx, s.repo, bm.UserId, bm.BusinessId, locationId, bm.IsPrimaryAdmin, bm.IsAdmin, role)
}

// generateTokens creates a JWT and a refresh token for a given user
func (s *AuthService) generateTokens(
	ctx context.Context,
	repo domain.Repo,
	userId,
	businessId,
	locationId string,
	isPrimaryAdmin,
	isAdmin bool,
	role domain.LRole,
) (*AuthResult, error) {
	// generate JWT
	accessToken, err := auth.GenerateAccessToken(userId, businessId, locationId, isPrimaryAdmin, isAdmin, role, s.jwtSecret)
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
	_, err = repo.CreateRefreshToken(ctx, userId, businessId, locationId, hashedToken, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &AuthResult{
		AccessToken:               accessToken,
		RefreshToken:              refreshToken,
		RequiresBusinessSelection: false,
	}, nil
}

func (s *AuthService) generateInterimAccessToken(userId string) (string, error) {
	return auth.GenerateAccessToken(userId, "", "", false, false, domain.EmptyRole, s.jwtSecret)
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
