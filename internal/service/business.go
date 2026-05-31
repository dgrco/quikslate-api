package service

import (
	"context"
	"fmt"

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

func (bs *BusinessService) GetBusiness(ctx context.Context, userId, businessId string) (domain.Business, error) {
	urole, err := bs.repo.GetUserRole(ctx, userId, businessId)
	if err != nil {
		return domain.Business{}, fmt.Errorf("failed to get user role: %w", err)
	}

	if urole.Role != domain.AdminRole {
		return domain.Business{}, domain.ErrUnauthorized
	}
	
	// b, err := bs.repo.Get
	return domain.Business{}, nil;
}
