package service

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

// EmployeeService manages which positions (job roles) a user is qualified
// to work within a business. A LocationLead or Manager may edit an
// employee's positions only if they share at least one location with that
// employee; anything outside that overlap requires a business admin.
type EmployeeService struct {
	repo domain.Repo
}

func NewEmployeeService(repo domain.Repo) *EmployeeService {
	return &EmployeeService{
		repo,
	}
}

func (es *EmployeeService) validateEmployeePositionAction(ctx context.Context, targetUserId string) error {
	if ctxkeys.GetIsAdmin(ctx) {
		return nil
	}

	businessId := ctxkeys.GetBusinessId(ctx)
	callerLocations, err := es.repo.GetLocationRolesByUserAndBusiness(ctx, ctxkeys.GetUserId(ctx), businessId)
	if err != nil {
		return err
	}
	targetLocations, err := es.repo.GetLocationRolesByUserAndBusiness(ctx, targetUserId, businessId)
	if err != nil {
		return err
	}
	for _, callerLoc := range callerLocations {
		if callerLoc.Role != domain.LocationLead && callerLoc.Role != domain.Manager {
			continue
		}
		for _, targetLoc := range targetLocations {
			if callerLoc.LocationId == targetLoc.LocationId {
				return nil
			}
		}
	}
	return domain.ErrForbidden
}

// AddPosition marks userId as qualified to work positionId. Authorization:
// Admin, or a LocationLead/Manager who shares a location with userId.
func (es *EmployeeService) AddPosition(
	ctx context.Context,
	userId,
	positionId string,
) error {
	if err := es.validateEmployeePositionAction(ctx, userId); err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	if _, err := es.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx)); err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	if _, err := getAndValidatePosition(ctx, es.repo, positionId); err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	if err := es.repo.AddPosition(ctx, userId, positionId); err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	return nil
}

// RemovePosition revokes userId's qualification for positionId.
// Authorization: Admin.
func (es *EmployeeService) RemovePosition(
	ctx context.Context,
	userId,
	positionId string,
) error {
	if err := validateIsAdmin(ctx); err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	if _, err := es.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx)); err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	if _, err := getAndValidatePosition(ctx, es.repo, positionId); err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	if err := es.repo.RemovePosition(ctx, userId, positionId); err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	return nil
}

// GetAllPositionsByUser lists every position userId is qualified to work
// within the caller's business. Authorization: Admin, or a
// LocationLead/Manager who shares a location with userId.
func (es *EmployeeService) GetAllPositionsByUser(
	ctx context.Context,
	userId string,
) ([]domain.EmployeePosition, error) {
	if err := es.validateEmployeePositionAction(ctx, userId); err != nil {
		return nil, fmt.Errorf("failed to get all positions by user: %w", err)
	}

	bm, err := es.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get all positions by user: %w", err)
	}

	positions, err := es.repo.GetPositionsByUserAndBusiness(ctx, userId, bm.BusinessId)
	if err != nil {
		return nil, fmt.Errorf("failed to get all positions by user: %w", err)
	}

	return positions, nil
}
