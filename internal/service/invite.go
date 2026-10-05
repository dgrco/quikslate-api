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

// InviteService lets an admin or LocationLead bring a new person into a
// business at a specific location and role, without that person needing an
// account first. Invites are single-use, expire after 7 days, and are
// identified to the invitee by an opaque token (never the DB id), hashed
// the same way refresh tokens are, see AuthService.
type InviteService struct {
	repo domain.Repo
}

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
// at locationId.
// (Authorization: Admin, or LocationLead at this location. Manager is
// excluded: this can bring an entirely new person into the business, a
// broader capability than Manager's role-assignment authority warrants.)
func (s *InviteService) CreateInvite(
	ctx context.Context,
	locationId string,
	email string,
	targetRole domain.LRole,
) (InviteResult, error) {
	businessId := ctxkeys.GetBusinessId(ctx)
	callerRole := resolveCallerRole(ctx)

	if callerRole != domain.Admin && callerRole != domain.LocationLead {
		return InviteResult{}, domain.ErrForbidden
	}

	if !canActOnRole(callerRole, targetRole) {
		return InviteResult{}, domain.ErrForbidden
	}

	_, err := s.repo.GetPendingInviteByEmailAndBusinessId(ctx, email, businessId)
	if err == nil {
		return InviteResult{}, fmt.Errorf("failed to create invite: %w", domain.ErrAlreadyExists)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return InviteResult{}, fmt.Errorf("failed to create invite: %w", err)
	}

	u, err := s.repo.GetUserByEmail(ctx, email)
	if err == nil {
		if _, err := s.repo.GetBusinessMember(ctx, u.Id, businessId); err == nil {
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

	email = domain.NormalizeEmail(email)
	if err := domain.ValidateEmail(email); err != nil {
		return InviteResult{}, fmt.Errorf("failed to create invite: %w", err)
	}
	if err := domain.ValidateNonAdminLocationRole(targetRole); err != nil {
		return InviteResult{}, fmt.Errorf("failed to create invite: %w", err)
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
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

// GetPendingInvites returns every not-yet-accepted invite at locationId,
// including ones another caller created, so an admin or LocationLead can
// see who's already been invited.
// (Authorization: admin, or LocationLead at this location)
func (s *InviteService) GetPendingInvites(ctx context.Context, locationId string) ([]domain.Invite, error) {
	callerRole := resolveCallerRole(ctx)
	if callerRole != domain.Admin && callerRole != domain.LocationLead {
		return nil, domain.ErrForbidden
	}

	invites, err := s.repo.GetPendingInvitesByLocationId(ctx, locationId)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending invites: %w", err)
	}
	return invites, nil
}

// RevokeInvite cancels a not-yet-accepted invite at locationId, freeing the
// email up to be invited again.
// (Authorization: admin, or LocationLead who outranks the invite's target
// role: a LocationLead can't revoke an invite for a role they couldn't
// have created themselves, e.g. one for a fellow LocationLead.)
func (s *InviteService) RevokeInvite(ctx context.Context, locationId, inviteId string) error {
	inv, err := s.repo.GetInviteById(ctx, inviteId)
	if err != nil {
		return fmt.Errorf("failed to revoke invite: %w", err)
	}
	if inv.LocationId != locationId {
		return domain.ErrNotFound
	}

	if !canActOnRole(resolveCallerRole(ctx), inv.Role) {
		return domain.ErrForbidden
	}

	if err := s.repo.DeleteInvite(ctx, inviteId); err != nil {
		return fmt.Errorf("failed to revoke invite: %w", err)
	}
	return nil
}

// AcceptInvite accepts an invite if validated and returns the associated businessId.
func (s *InviteService) AcceptInvite(ctx context.Context, token string) (string, error) {
	userId := ctxkeys.GetUserId(ctx)

	inv, err := s.repo.GetInviteByTokenHash(ctx, hashToken(token))
	if err != nil {
		return "", domain.ErrNotFound
	}
	if inv.AcceptedAt != nil || time.Now().After(inv.ExpiresAt) {
		return "", domain.ErrNotFound
	}

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
