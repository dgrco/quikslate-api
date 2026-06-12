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

type AuthResponse struct {
	AccessToken               string                `json:"access_token,omitempty"`
	RefreshToken              string                `json:"refresh_token,omitempty"`

	// Below are fields used for business/location selection
	Businesses                []domain.Business     `json:"businesses,omitempty"`
	Locations                 []domain.LocationRole `json:"locations,omitempty"`
	UserId										string 								`json:"user_id,omitempty"`
	RequiresBusinessSelection bool                  `json:"requires_business_selection"`
	RequiresLocationSelection bool                  `json:"requires_location_selection"`
}

func (s *AuthService) Register(ctx context.Context, email, password, businessName string) (*AuthResponse, error) {
	// validate email and password
	if err := domain.ValidateUserRegistrationCredentials(email, password); err != nil {
		return nil, err
	}

	// validate businessName
	if err := domain.ValidateBusinessName(businessName); err != nil {
		return nil, err
	}

	// hash password
	hashed, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// create user & business
	// TODO: separate creating user w/ business and non-admin registrations
	user, business, err := s.repo.CreateUserWithBusiness(ctx, email, hashed, businessName)
	if err != nil {
		return nil, fmt.Errorf("failed to create user with business: %w", err)
	}

	// generate tokens
	// this is a new user + business registration -> set new user to admin
	return s.generateTokens(ctx, user.Id, business.Id, domain.EmptyLocation, true, domain.EmptyRole)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
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

	if len(bms) == 0 {
		return nil, fmt.Errorf("login error: user belongs to no business")
	}

	if len(bms) == 1 {
		return s.SelectBusiness(ctx, user.Id, bms[0].BusinessId)
	}

	// user belongs to more than one business
	businesses := []domain.Business{}
	for _, bm := range bms {
		b, err := s.repo.GetBusinessById(ctx, bm.BusinessId)
		if err != nil {
			return nil, fmt.Errorf("login error: %w", err)
		}
		businesses = append(businesses, b)
	}

	return &AuthResponse{
		Businesses:                businesses,
		RequiresBusinessSelection: true,
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*AuthResponse, error) {
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

	// rotate the token: delete old one, issue new one
	if err := s.repo.DeleteRefreshToken(ctx, hashedToken); err != nil {
		return nil, fmt.Errorf("failed to rotate refresh token: %w", err)
	}

	// fetch business member data for the user
	bm, err := s.repo.GetBusinessMember(ctx, stored.UserId, stored.BusinessId)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	locationId := stored.LocationId
	var role domain.LRole
	if stored.LocationId != "" && !bm.IsAdmin {
		lr, err := s.repo.GetLocationRole(ctx, stored.UserId, stored.LocationId)
		if err != nil {
			return nil, domain.ErrNotFound
		}
		role = lr.Role
	}

	return s.generateTokens(ctx, bm.UserId, bm.BusinessId, locationId, bm.IsAdmin, role)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	hashedToken := hashToken(refreshToken)
	return s.repo.DeleteRefreshToken(ctx, hashedToken)
}

// Requires explicit userId, businessId since this can be called at login-time (context not yet filled!)
func (s *AuthService) SelectBusiness(ctx context.Context, userId, businessId string) (*AuthResponse, error) {
	bm, err := s.repo.GetBusinessMember(ctx, userId, businessId)
	if err != nil {
		return nil, fmt.Errorf("failed to select business: %w", err)
	}

	lrs, err := s.repo.GetLocationRolesByUserAndBusiness(ctx, userId, businessId)
	if err != nil {
		return nil, fmt.Errorf("failed to select business: %w", err)
	}

	if bm.IsAdmin && len(lrs) == 0 {
		return s.generateTokens(ctx, userId, businessId, domain.EmptyLocation, bm.IsAdmin, domain.EmptyRole)
	} else if len(lrs) == 1 {
		return s.generateTokens(ctx, userId, businessId, lrs[0].LocationId, bm.IsAdmin, lrs[0].Role)
	} else {
		// Many locations -> requires selection
		return &AuthResponse{Locations: lrs, RequiresLocationSelection: true}, nil
	}
}

// Requires explicit userId, businessId, locationId since this can be called at login-time (context not yet filled!)
func (s *AuthService) SelectLocation(ctx context.Context, userId, businessId, locationId string) (*AuthResponse, error) {
	bm, err := s.repo.GetBusinessMember(ctx, userId, businessId)
	if err != nil {
		return nil, fmt.Errorf("failed to select location: %w", err)
	}

	role := domain.EmptyRole
	if !bm.IsAdmin {
		lr, err := s.repo.GetLocationRole(ctx, userId, locationId)
		if err != nil {
			return nil, fmt.Errorf("failed to select location: %w", err)
		}
		role = lr.Role
	}
	return s.generateTokens(ctx, bm.UserId, bm.BusinessId, locationId, bm.IsAdmin, role)
}

// generateTokens creates a JWT and a refresh token for a given user
func (s *AuthService) generateTokens(ctx context.Context, userId, businessId, locationId string, isAdmin bool, role domain.LRole) (*AuthResponse, error) {
	// generate JWT
	accessToken, err := auth.GenerateJWT(userId, businessId, locationId, isAdmin, role, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// generate a random refresh token
	refreshToken, err := generateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token")
	}

	// hash the refresh token for storage
	hashedToken := hashToken(refreshToken)

	// store hashed refresh token in database
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	_, err = s.repo.CreateRefreshToken(ctx, userId, businessId, locationId, hashedToken, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &AuthResponse{
		AccessToken:               accessToken,
		RefreshToken:              refreshToken,
		RequiresBusinessSelection: false,
	}, nil
}

// generateSecureToken generates 32 random bytes and hex encodes each,
// resulting in a string of length 64.
func generateSecureToken() (string, error) {
	bytes := make([]byte, 32)
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
