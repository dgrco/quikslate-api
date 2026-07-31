package service

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

type LocationService struct {
	repo domain.Repo
}

func NewLocationService(repo domain.Repo) *LocationService {
	return &LocationService{
		repo,
	}
}

// Create a Location and associate it with a businessId.
// The location must have a name, and it may optionally contain an address.
// (Authorization: admin)
func (ls *LocationService) CreateLocation(
	ctx context.Context,
	locationName string,
	address *string, // optional
) (domain.Location, error) {
	if err := validateIsAdmin(ctx); err != nil {
		return domain.Location{}, fmt.Errorf("failed to create location: %w", err)
	}

	if err := domain.ValidateLocationName(locationName); err != nil {
		return domain.Location{}, fmt.Errorf("failed to create location: %w", err)
	}

	l, err := ls.repo.CreateLocation(ctx, ctxkeys.GetBusinessId(ctx), locationName, address)
	if err != nil {
		return domain.Location{}, fmt.Errorf("failed to create location: %w", err)
	}
	return l, nil
}

// Get a Location given a locationId
// (Authorization: any business member)
func (ls *LocationService) GetLocation(
	ctx context.Context,
	locationId string,
) (domain.Location, error) {
	l, err := getAndValidateLocation(ctx, ls.repo, locationId)
	if err != nil {
		return domain.Location{}, fmt.Errorf("failed to get location: %w", err)
	}
	return l, nil
}

// Get all Locations associated to a businessId
// (Authorization: any business member)
func (ls *LocationService) GetAllLocations(
	ctx context.Context,
) ([]domain.Location, error) {
	locations, err := ls.repo.GetLocationsByBusinessId(ctx, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get all locations: %w", err)
	}
	return locations, nil
}

// Update a Location with locationId with a partial update object
// (Authorization: admin)
func (ls *LocationService) UpdateLocation(
	ctx context.Context,
	locationId string,
	update domain.LocationUpdate,
) error {
	if err := validateIsAdmin(ctx); err != nil {
		return fmt.Errorf("failed to update location: %w", err)
	}
	if update.Name != nil {
		if err := domain.ValidateLocationName(*update.Name); err != nil {
			return fmt.Errorf("failed to update location: %w", err)
		}
	}

	_, err := getAndValidateLocation(ctx, ls.repo, locationId)
	if err != nil {
		return fmt.Errorf("failed to update location: %w", err)
	}

	if err := ls.repo.UpdateLocationById(ctx, locationId, update); err != nil {
		return fmt.Errorf("failed to update location: %w", err)
	}
	return nil
}

// Delete a Location by its locationId
// (Authorization: admin)
func (ls *LocationService) DeleteLocation(
	ctx context.Context,
	locationId string,
) error {
	if err := validateIsAdmin(ctx); err != nil {
		return fmt.Errorf("failed to delete location: %w", err)
	}

	_, err := getAndValidateLocation(ctx, ls.repo, locationId)
	if err != nil {
		return fmt.Errorf("failed to delete location: %w", err)
	}

	if err := ls.repo.DeleteLocation(ctx, locationId); err != nil {
		return fmt.Errorf("failed to delete location: %w", err)
	}
	return nil
}
