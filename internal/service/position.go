package service

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

type PositionService struct {
	repo domain.Repo
}

func NewPositionService(repo domain.Repo) *PositionService {
	return &PositionService{
		repo,
	}
}

// Create a Position
// (Authorization: admin)
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

// Get a Position
// (Authorization: any business member)
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

// Get all Positions that exist in a Business
// (Authorization: any business member)
func (ps *PositionService) GetAllPositionsByBusiness(ctx context.Context) ([]domain.Position, error) {
	positions, err := ps.repo.GetPositionsByBusinessId(ctx, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get all positions by business: %w", err)
	}

	return positions, nil
}

// Rename a Position at a Business
// (Authorization: admin)
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

// Delete a Position at a Business
// (Authorization: admin)
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
