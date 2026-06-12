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

func NewBusinessService(repo domain.Repo) *BusinessService {
	return &BusinessService{
		repo,
	}
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

// Rename a Business.
// Implicit parameters set by http context: {businessId}
// (Authorization: admin)
func (bs *BusinessService) RenameBusiness(ctx context.Context, businessName string) error {
	if err := validateIsAdmin(ctx); err != nil {
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
