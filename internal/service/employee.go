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
// (Authorization: Admin, Manager)
func (es *EmployeeService) AddPosition(
	ctx context.Context,
	userId,
	locationId,
	positionId string,
) error {
	// validate the requestor has authorization
	if err := validateAdminOrLocationRole(ctx, es.repo, locationId, []domain.LRole{domain.Manager}); err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	// check if userId belongs to the same business
	_, err := es.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	if err := es.repo.AddPosition(ctx, userId, positionId); err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	return nil
}

// Remove a Position from a User
// (Authorization: Admin, Manager)
func (es *EmployeeService) RemovePosition(
	ctx context.Context,
	locationId,
	userId,
	positionId string,
) error {
	// validate the requestor has authorization
	if err := validateAdminOrLocationRole(ctx, es.repo, locationId, []domain.LRole{domain.Manager}); err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	// check if userId belongs to the same business
	_, err := es.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	if err := es.repo.RemovePosition(ctx, userId, positionId); err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	return nil
}

// Get all Positions at a Location for a User
// (Authorization: Admin, Manager)
func (es *EmployeeService) GetAllPositionsByUser(
	ctx context.Context,
	locationId,
	userId string,
) ([]domain.EmployeePosition, error) {
	// validate the requestor has authorization
	if err := validateAdminOrLocationRole(ctx, es.repo, locationId, []domain.LRole{domain.Manager}); err != nil {
		return nil, fmt.Errorf("failed to get all positions by user: %w", err)
	}

	// check if userId belongs to the same business
	_, err := es.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get all positions by user: %w", err)
	}

	positions, err := es.repo.GetPositionsByUserId(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get all positions by user: %w", err)
	}

	return positions, nil
}
