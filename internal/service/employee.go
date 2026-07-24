package service

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

type EmployeeService struct {
	repo domain.Repo
}

func NewEmployeeService(repo domain.Repo) *EmployeeService {
	return &EmployeeService{
		repo,
	}
}

// Add a Position to a User.
// (Authorization: Admin, LocationLead, Manager)
func (es *EmployeeService) AddPosition(
	ctx context.Context,
	userId,
	positionId string,
) error {
	callerLocationId, err := requireAdminOrManagerAtOwnLocation(ctx)
	if err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	// check if userId belongs to the same business (for admin callers)
	bm, err := es.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	if callerLocationId != "" {
		// verify manager works at the same location as the target
		if _, err := es.repo.GetLocationRole(ctx, userId, callerLocationId, bm.BusinessId); err != nil {
			return fmt.Errorf("failed to add position to employee: %w", err)
		}
	}

	// verify positionId belongs to the caller's business
	_, err = getAndValidatePosition(ctx, es.repo, positionId)
	if err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	if err := es.repo.AddPosition(ctx, userId, positionId); err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	return nil
}

// Remove a Position from a User
// (Authorization: Admin, LocationLead, Manager)
func (es *EmployeeService) RemovePosition(
	ctx context.Context,
	userId,
	positionId string,
) error {
	callerLocationId, err := requireAdminOrManagerAtOwnLocation(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	// check if userId belongs to the same business (for admin callers)
	bm, err := es.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	if callerLocationId != "" {
		// verify manager works at the same location as the target
		if _, err := es.repo.GetLocationRole(ctx, userId, callerLocationId, bm.BusinessId); err != nil {
			return fmt.Errorf("failed to remove position from employee: %w", err)
		}
	}
	
	// verify positionId belongs to the caller's business
	_, err = getAndValidatePosition(ctx, es.repo, positionId)
	if err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	if err := es.repo.RemovePosition(ctx, userId, positionId); err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	return nil
}

// Get all Positions at a Business for a User
// (Authorization: Admin, LocationLead, Manager)
func (es *EmployeeService) GetAllPositionsByUser(
	ctx context.Context,
	userId string,
) ([]domain.EmployeePosition, error) {
	callerLocationId, err := requireAdminOrManagerAtOwnLocation(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all positions by user: %w", err)
	}

	// check if userId belongs to the same business (for admin callers)
	bm, err := es.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get all positions by user: %w", err)
	}

	if callerLocationId != "" {
		// verify manager works at the same location as the target
		if _, err := es.repo.GetLocationRole(ctx, userId, callerLocationId, bm.BusinessId); err != nil {
			return nil, fmt.Errorf("failed to get all positions by user: %w", err)
		}
	}

	positions, err := es.repo.GetPositionsByUserAndBusiness(ctx, userId, bm.BusinessId)
	if err != nil {
		return nil, fmt.Errorf("failed to get all positions by user: %w", err)
	}

	return positions, nil
}
