package service

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

// BusinessService implements the business-management use cases: creating a
// business, renaming or deleting it, and managing membership (admin status,
// removal). Most methods expect businessId, and often userId, to already be
// resolved into ctx by internal/handler's auth middleware.
type BusinessService struct {
	repo domain.Repo
}

func NewBusinessService(repo domain.Repo) *BusinessService {
	return &BusinessService{
		repo,
	}
}

// CreateBusiness creates a new business and adds the caller as its primary
// admin, in one transaction. Any authenticated user may create a business.
func (bs *BusinessService) CreateBusiness(ctx context.Context, name string) (string, error) {
	if err := domain.ValidateBusinessName(name); err != nil {
		return "", fmt.Errorf("failed to create business: %w", err)
	}

	userId := ctxkeys.GetUserId(ctx)

	tx, err := bs.repo.BeginTransaction(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create business: %w", err)
	}
	defer tx.Rollback(ctx)

	txRepo := bs.repo.WithTx(tx)

	b, err := txRepo.CreateBusiness(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to create business: %w", err)
	}

	if err := txRepo.AddUserToBusiness(ctx, userId, b.Id, true, true); err != nil {
		return "", fmt.Errorf("failed to create business: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("failed to create business: %w", err)
	}

	return b.Id, nil
}

// GetBusinessesByUserId returns every business the caller belongs to.
// Authorization: any authenticated user; the user ID comes from ctx, so
// callers can only query their own membership.
func (bs *BusinessService) GetBusinessesByUserId(ctx context.Context) ([]domain.Business, error) {
	businesses, err := bs.repo.GetBusinessesByUserId(ctx, ctxkeys.GetUserId(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get businesses by user ID: %w", err)
	}

	return businesses, nil
}

// GetBusiness returns businessId's details. Authorization: admin.
func (bs *BusinessService) GetBusiness(ctx context.Context) (domain.Business, error) {
	if err := validateIsAdmin(ctx); err != nil {
		return domain.Business{}, fmt.Errorf("failed to get business: %w", err)
	}
	businessId := ctxkeys.GetBusinessId(ctx)

	b, err := bs.repo.GetBusinessById(ctx, businessId)
	if err != nil {
		return domain.Business{}, fmt.Errorf("failed to get business: %w", err)
	}
	return b, nil
}

// MyBusinessMembership is the caller's own standing within a business: their
// admin status (read straight from context, already computed fresh by
// RequireBusinessMember for this request, so no extra DB call) plus every
// location role they hold within it. It exists so the frontend can decide
// what admin-only or location-scoped UI to show without probing each action
// individually and reacting to 403s.
type MyBusinessMembership struct {
	IsAdmin        bool
	IsPrimaryAdmin bool
	LocationRoles  []domain.LocationRole
}

// GetMyBusinessMembership returns the caller's own admin status and location
// roles for businessId. Authorization: any business member, since this is a
// self-lookup rather than a listing of others.
func (bs *BusinessService) GetMyBusinessMembership(ctx context.Context) (MyBusinessMembership, error) {
	businessId := ctxkeys.GetBusinessId(ctx)

	roles, err := bs.repo.GetLocationRolesByUserAndBusiness(ctx, ctxkeys.GetUserId(ctx), businessId)
	if err != nil {
		return MyBusinessMembership{}, fmt.Errorf("failed to get my business membership: %w", err)
	}

	return MyBusinessMembership{
		IsAdmin:        ctxkeys.GetIsAdmin(ctx),
		IsPrimaryAdmin: ctxkeys.GetIsPrimaryAdmin(ctx),
		LocationRoles:  roles,
	}, nil
}

// GetBusinessMemberDetailsByBusinessId returns every member of businessId,
// joined with their name and email for display. Authorization: admin.
func (bs *BusinessService) GetBusinessMemberDetailsByBusinessId(ctx context.Context) ([]domain.BusinessMemberDetail, error) {
	if err := validateIsAdmin(ctx); err != nil {
		return nil, fmt.Errorf("failed to get business member details by business ID: %w", err)
	}
	businessId := ctxkeys.GetBusinessId(ctx)

	bmds, err := bs.repo.GetBusinessMemberDetailsByBusinessId(ctx, businessId)
	if err != nil {
		return nil, fmt.Errorf("failed to get business member details by business ID: %w", err)
	}
	return bmds, nil
}

// RenameBusiness changes businessId's name. Authorization: admin.
func (bs *BusinessService) RenameBusiness(ctx context.Context, businessName string) error {
	if err := validateIsAdmin(ctx); err != nil {
		return fmt.Errorf("failed to rename business: %w", err)
	}
	if err := domain.ValidateBusinessName(businessName); err != nil {
		return fmt.Errorf("failed to rename business: %w", err)
	}

	businessId := ctxkeys.GetBusinessId(ctx)

	if err := bs.repo.ChangeBusinessName(ctx, businessId, businessName); err != nil {
		return fmt.Errorf("failed to rename business: %w", err)
	}
	return nil
}

// DeleteBusiness deletes businessId. Authorization: admin.
func (bs *BusinessService) DeleteBusiness(ctx context.Context) error {
	if err := validateIsAdmin(ctx); err != nil {
		return fmt.Errorf("failed to delete business: %w", err)
	}
	businessId := ctxkeys.GetBusinessId(ctx)

	if err := bs.repo.DeleteBusiness(ctx, businessId); err != nil {
		return fmt.Errorf("failed to delete business: %w", err)
	}
	return nil
}

// RemoveUserFromBusiness removes userId from businessId, along with every
// location role and position they held there. Refuses to remove the last
// remaining admin. Authorization: admin, and the caller must
// canActOnBusinessMember the target.
func (s *BusinessService) RemoveUserFromBusiness(ctx context.Context, userId string) error {
	if err := validateIsAdmin(ctx); err != nil {
		return fmt.Errorf("failed to remove user from business: %w", err)
	}

	tx, err := s.repo.BeginTransaction(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	txRepo := s.repo.WithTx(tx)

	businessId := ctxkeys.GetBusinessId(ctx)

	bm, err := txRepo.GetBusinessMember(ctx, userId, businessId)
	if err != nil {
		return fmt.Errorf("failed to remove user from business: %w", err)
	}

	if !canActOnBusinessMember(ctx, &bm) {
		return domain.ErrForbidden
	}

	if bm.IsAdmin {
		nAdmins, err := txRepo.GetAdminCount(ctx, businessId)
		if err != nil {
			return fmt.Errorf("failed to remove user from business: %w", err)
		}
		if nAdmins <= 1 {
			return domain.ErrLastAdminRemoval
		}
	}

	if err := txRepo.RemoveUserFromBusiness(ctx, userId, businessId); err != nil {
		return fmt.Errorf("failed to remove user from business: %w", err)
	}

	if err := txRepo.RemoveAllRolesOfUserFromBusiness(ctx, userId, businessId); err != nil {
		return fmt.Errorf("failed to remove user from business: %w", err)
	}

	if err := txRepo.RemoveAllPositionsForUserInBusiness(ctx, userId, businessId); err != nil {
		return fmt.Errorf("failed to remove user from business: %w", err)
	}

	return tx.Commit(ctx)
}

// SetAdminForBusinessMember promotes or demotes userId's admin status
// within businessId. Refuses to demote the last remaining admin.
// Authorization: admin, and the caller must canActOnBusinessMember the
// target.
func (s *BusinessService) SetAdminForBusinessMember(ctx context.Context, userId string, admin bool) error {
	if err := validateIsAdmin(ctx); err != nil {
		return fmt.Errorf("failed to remove user from business: %w", err)
	}

	tx, err := s.repo.BeginTransaction(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	txRepo := s.repo.WithTx(tx)

	businessId := ctxkeys.GetBusinessId(ctx)

	bm, err := txRepo.GetBusinessMember(ctx, userId, businessId)
	if err != nil {
		return fmt.Errorf("failed to set admin: %w", err)
	}

	if !canActOnBusinessMember(ctx, &bm) {
		return domain.ErrForbidden
	}

	if bm.IsAdmin && admin == false {
		nAdmins, err := txRepo.GetAdminCount(ctx, businessId)
		if err != nil {
			return fmt.Errorf("failed to set admin: %w", err)
		}
		if nAdmins <= 1 {
			return domain.ErrLastAdminRemoval
		}
	}

	if err := txRepo.SetAdminForBusinessMember(ctx, userId, businessId, admin); err != nil {
		return fmt.Errorf("failed to set admin: %w", err)
	}

	return tx.Commit(ctx)
}
