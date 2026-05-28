package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/pkg/auth"
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
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (s *AuthService) Register(ctx context.Context, email, password string) (*AuthResponse, error) {
	// validate email and password
	err := domain.ValidateRegistrationCredentials(email, password)
	if err != nil {
		return nil, err
	}

	// check if user already exists
	_, err = s.repo.GetUserByEmail(ctx, email)
	if err == nil {
		return nil, domain.ErrAlreadyExists
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	// hash password
	hashed, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// create user
	user, err := s.repo.CreateUser(ctx, email, hashed)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// generate tokens
	return s.generateTokens(ctx, user.ID)
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

	return s.generateTokens(ctx, user.ID)
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
		s.repo.DeleteRefreshToken(ctx, hashedToken)
		return nil, domain.ErrInvalidRefreshToken
	}

	// rotate the token: delete old one, issue new one
	if err := s.repo.DeleteRefreshToken(ctx, hashedToken); err != nil {
		return nil, fmt.Errorf("failed to rotate refresh token: %w", err)
	}

	return s.generateTokens(ctx, stored.UserID)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	hashedToken := hashToken(refreshToken)
	return s.repo.DeleteRefreshToken(ctx, hashedToken)
}

// generateTokens creates a JWT and a refresh token for a given user
func (s *AuthService) generateTokens(ctx context.Context, userID string) (*AuthResponse, error) {
	// generate JWT
	accessToken, err := auth.GenerateJWT(userID, s.jwtSecret)
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
	_, err = s.repo.CreateRefreshToken(ctx, userID, hashedToken, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
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
