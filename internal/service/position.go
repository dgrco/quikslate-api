package service

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

// PositionService manages the job roles (e.g. "cashier") a business offers,
// used to categorize shifts and to record which positions an employee is
// qualified to work.
type PositionService struct {
	repo domain.Repo
}

func NewPositionService(repo domain.Repo) *PositionService {
	return &PositionService{
		repo,
	}
}

// CreatePosition adds a new position to the caller's business.
// Authorization: admin.
func (ps *PositionService) CreatePosition(
	ctx context.Context,
	positionName string,
) (domain.Position, error) {
	if err := validateIsAdmin(ctx); err != nil {
		return domain.Position{}, fmt.Errorf("failed to create position: %w", err)
	}

	if err := domain.ValidatePositionName(positionName); err != nil {
		return domain.Position{}, fmt.Errorf("failed to create position: %w", err)
	}

	p, err := ps.repo.CreatePosition(ctx, ctxkeys.GetBusinessId(ctx), positionName)
	if err != nil {
		return domain.Position{}, fmt.Errorf("failed to create position: %w", err)
	}

	return p, nil
}

// GetPosition returns positionId's details. Authorization: any business
// member.
func (ps *PositionService) GetPosition(
	ctx context.Context,
	positionId string,
) (domain.Position, error) {
	p, err := getAndValidatePosition(ctx, ps.repo, positionId)
	if err != nil {
		return domain.Position{}, fmt.Errorf("failed to get position: %w", err)
	}

	return p, nil
}

// GetAllPositionsByBusiness lists every position the caller's business
// offers. Authorization: any business member.
func (ps *PositionService) GetAllPositionsByBusiness(ctx context.Context) ([]domain.Position, error) {
	positions, err := ps.repo.GetPositionsByBusinessId(ctx, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get all positions by business: %w", err)
	}

	return positions, nil
}

// RenamePosition changes positionId's name. Authorization: admin.
func (ps *PositionService) RenamePosition(
	ctx context.Context,
	positionId,
	positionName string,
) error {
	if err := validateIsAdmin(ctx); err != nil {
		return fmt.Errorf("failed to rename position: %w", err)
	}

	_, err := getAndValidatePosition(ctx, ps.repo, positionId)
	if err != nil {
		return fmt.Errorf("failed to rename position: %w", err)
	}

	if err := domain.ValidatePositionName(positionName); err != nil {
		return fmt.Errorf("failed to rename position: %w", err)
	}

	if err := ps.repo.ChangePositionName(ctx, positionId, positionName); err != nil {
		return fmt.Errorf("failed to rename position: %w", err)
	}

	return nil
}

// DeletePosition deletes positionId. Authorization: admin.
func (ps *PositionService) DeletePosition(
	ctx context.Context,
	positionId string,
) error {
	if err := validateIsAdmin(ctx); err != nil {
		return fmt.Errorf("failed to delete position: %w", err)
	}

	_, err := getAndValidatePosition(ctx, ps.repo, positionId)
	if err != nil {
		return fmt.Errorf("failed to delete position: %w", err)
	}

	if err := ps.repo.DeletePosition(ctx, positionId); err != nil {
		return fmt.Errorf("failed to delete position: %w", err)
	}

	return nil
}
