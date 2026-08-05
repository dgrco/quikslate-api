package service

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

type BusinessService struct {
	repo domain.Repo
}

// NewBusinessService constructs a BusinessService backed by repo.
func NewBusinessService(repo domain.Repo) *BusinessService {
	return &BusinessService{
		repo,
	}
}

// Create a Business.
// Anyone authenticated can make a business.
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

// Get all businesses a user belongs to
// Implicit parameters set by http context: {userId}
// (Authorization: all, but only the user may query their own)
func (bs *BusinessService) GetBusinessesByUserId(ctx context.Context) ([]domain.Business, error) {
	businesses, err := bs.repo.GetBusinessesByUserId(ctx, ctxkeys.GetUserId(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get businesses by user ID: %w", err)
	}

	return businesses, nil
}

// Get a Business.
// Implicit parameters set by http context: {businessId}
// (Authorization: admin)
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

// Get Business Member Details for every user in a business.
// Implicit parameters set by http context: {businessId}
// (Authorization: admin)
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

// Rename a Business.
// Implicit parameters set by http context: {businessId}
// (Authorization: admin)
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

// Delete a Business.
// Implicit parameters set by http context: {businessId}
// (Authorization: admin)
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

// Remove a User from a Business.
// Implicit parameters set by http context: {businessId}
// (Authorization: admin)
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

	// Check if the caller can remove the target
	if !canActOnBusinessMember(ctx, &bm) {
		return domain.ErrForbidden
	}

	// Last admin removal check
	if bm.IsAdmin {
		nAdmins, err := txRepo.GetAdminCount(ctx, businessId)
		if err != nil {
			return fmt.Errorf("failed to remove user from business: %w", err)
		}
		if nAdmins <= 1 {
			return domain.ErrLastAdminRemoval
		}
	}

	// Remove the user from business_members
	if err := txRepo.RemoveUserFromBusiness(ctx, userId, businessId); err != nil {
		return fmt.Errorf("failed to remove user from business: %w", err)
	}

	// Remove every location role the user has at the business
	if err := txRepo.RemoveAllRolesOfUserFromBusiness(ctx, userId, businessId); err != nil {
		return fmt.Errorf("failed to remove user from business: %w", err)
	}

	// Remove all employee_positions entries associated with the user and the business
	if err := txRepo.RemoveAllPositionsForUserInBusiness(ctx, userId, businessId); err != nil {
		return fmt.Errorf("failed to remove user from business: %w", err)
	}

	return tx.Commit(ctx)
}

// Sets the admin status of a User.
// Implicit parameters from context: {businessId}
// (Authorization: admin)
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

	// Check if the caller can modify the target
	if !canActOnBusinessMember(ctx, &bm) {
		return domain.ErrForbidden
	}

	// Last admin removal check
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
