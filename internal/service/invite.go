package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

type InviteService struct {
	repo domain.Repo
}

// NewInviteService constructs an InviteService backed by repo.
func NewInviteService(repo domain.Repo) *InviteService {
	return &InviteService{
		repo,
	}
}

type InviteResult struct {
	InviteToken string `json:"invite_token,omitempty"`
}

type InviteDTO struct {
	Email        string       `json:"email"`
	BusinessName string       `json:"business_name"`
	Role         domain.LRole `json:"role"`
	ExpiresAt    time.Time    `json:"expires_at"`
}

// CreateInvite creates an outstanding invite to a (potential) user's email
// (Authorization: Admin, or LocationLead at the target location)
func (s *InviteService) CreateInvite(
	ctx context.Context,
	email string,
	locationId string,
	targetRole domain.LRole,
) (InviteResult, error) {
	businessId := ctxkeys.GetBusinessId(ctx)

	// This route is business-scoped (RequireBusinessMember), so ctxkeys.Role/
	// LocationId are never set — look up the caller's role at the invite's
	// target location directly instead of trusting session context.
	l, err := s.repo.GetLocationById(ctx, locationId)
	if err != nil {
		return InviteResult{}, fmt.Errorf("failed to create invite: %w", err)
	}
	if l.BusinessId != businessId {
		return InviteResult{}, fmt.Errorf("failed to create invite: %w", domain.ErrForbidden)
	}

	callerRole := domain.Admin
	if !ctxkeys.GetIsAdmin(ctx) {
		lr, err := s.repo.GetLocationRole(ctx, ctxkeys.GetUserId(ctx), locationId, businessId)
		if err != nil || lr.Role != domain.LocationLead {
			return InviteResult{}, fmt.Errorf("failed to create invite: %w", domain.ErrForbidden)
		}
		callerRole = lr.Role
	}

	// Enforce caller -> target role hierarchy
	if !canActOnRole(callerRole, targetRole) {
		return InviteResult{}, domain.ErrForbidden
	}

	// Precondition check: user must not already have a pending invite
	_, err = s.repo.GetPendingInviteByEmailAndBusinessId(ctx, email, businessId)
	if err == nil {
		// pending invite already exists
		return InviteResult{}, fmt.Errorf("failed to create invite: %w", domain.ErrAlreadyExists)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return InviteResult{}, fmt.Errorf("failed to create invite: %w", err)
	}

	// Precondition check: user must not already be a member of the business
	u, err := s.repo.GetUserByEmail(ctx, email)
	if err == nil {
		// Member with this email already exists, verify it's not a member of the business
		if _, err := s.repo.GetBusinessMember(ctx, u.Id, businessId); err == nil {
			// User already exists!
			return InviteResult{}, fmt.Errorf("failed to create invite: %w", domain.ErrAlreadyExists)
		}
	}
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return InviteResult{}, fmt.Errorf("failed to create invite: %w", err)
	}

	token, err := generateSecureToken(32)
	if err != nil {
		return InviteResult{}, fmt.Errorf("failed to create invite: %w", err)
	}
	tokenHash := hashToken(token)

	// Validation
	email = domain.NormalizeEmail(email)
	if err := domain.ValidateEmail(email); err != nil {
		return InviteResult{}, fmt.Errorf("failed to create invite: %w", err)
	}
	if err := domain.ValidateNonAdminLocationRole(targetRole); err != nil {
		return InviteResult{}, fmt.Errorf("failed to create invite: %w", err)
	}

	// Call repo function
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days
	_, err = s.repo.CreateInvite(ctx, tokenHash, email, businessId, locationId, targetRole, expiresAt)
	if err != nil {
		return InviteResult{}, fmt.Errorf("failed to create invite: %w", err)
	}

	return InviteResult{InviteToken: token}, nil
}

// PreviewInviteByToken is callable by anyone with the token.
func (s *InviteService) PreviewInviteByToken(ctx context.Context, token string) (InviteDTO, error) {
	tokenHash := hashToken(token)

	inv, err := s.repo.GetInviteByTokenHash(ctx, tokenHash)
	if err != nil {
		return InviteDTO{}, fmt.Errorf("failed to get invite by token: %w", err)
	}

	if inv.AcceptedAt != nil || time.Now().After(inv.ExpiresAt) {
		return InviteDTO{}, domain.ErrNotFound
	}

	// Get business to extract businessName
	b, err := s.repo.GetBusinessById(ctx, inv.BusinessId)
	if err != nil {
		return InviteDTO{}, fmt.Errorf("failed to get invite by token: %w", err)
	}

	dto := InviteDTO{
		Email:        inv.Email,
		BusinessName: b.Name,
		Role:         inv.Role,
		ExpiresAt:    inv.ExpiresAt,
	}

	return dto, nil
}

// AcceptInvite accepts an invite if validated and returns the associated businessId.
func (s *InviteService) AcceptInvite(ctx context.Context, token string) (string, error) {
	userId := ctxkeys.GetUserId(ctx)

	// Token validation
	inv, err := s.repo.GetInviteByTokenHash(ctx, hashToken(token))
	if err != nil {
		return "", domain.ErrNotFound
	}
	if inv.AcceptedAt != nil || time.Now().After(inv.ExpiresAt) {
		return "", domain.ErrNotFound
	}

	// Confirm user email and invite email matches
	caller, err := s.repo.GetUserById(ctx, userId)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(caller.Email, inv.Email) {
		return "", domain.ErrForbidden
	}

	tx, err := s.repo.BeginTransaction(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txRepo := s.repo.WithTx(tx)

	// Set-up the user in the business/location
	// and mark the invite as accepted.
	if err := txRepo.AddUserToBusiness(ctx, userId, inv.BusinessId, false, false); err != nil {
		return "", err
	}
	if err := txRepo.AssignRole(ctx, userId, inv.BusinessId, inv.LocationId, inv.Role); err != nil {
		return "", err
	}
	if err := txRepo.MarkInviteAccepted(ctx, hashToken(token)); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return inv.BusinessId, nil
}
