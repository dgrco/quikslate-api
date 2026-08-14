package service

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

// LocationService manages a business's locations: the physical sites where
// positions are offered and shifts are scheduled.

type LocationService struct {
	repo domain.Repo
}

// NewLocationService constructs a LocationService backed by repo.
func NewLocationService(repo domain.Repo) *LocationService {
	return &LocationService{
		repo,
	}
}

// CreateLocation adds a new location to the caller's business. It must have
// a name and an IANA timezone; the address is optional. Authorization:
// admin.
func (ls *LocationService) CreateLocation(
	ctx context.Context,
	locationName string,
	address *string, // optional
	timezone string,
) (domain.Location, error) {
	if err := validateIsAdmin(ctx); err != nil {
		return domain.Location{}, fmt.Errorf("failed to create location: %w", err)
	}

	if err := domain.ValidateLocationName(locationName); err != nil {
		return domain.Location{}, fmt.Errorf("failed to create location: %w", err)
	}

	if address != nil {
		if err := domain.ValidateLocationAddr(*address); err != nil {
			return domain.Location{}, fmt.Errorf("failed to create location: %w", err)
		}
	}

	if err := domain.ValidateTimezone(timezone); err != nil {
		return domain.Location{}, fmt.Errorf("failed to create location: %w", err)
	}

	l, err := ls.repo.CreateLocation(ctx, ctxkeys.GetBusinessId(ctx), locationName, address, timezone)
	if err != nil {
		return domain.Location{}, fmt.Errorf("failed to create location: %w", err)
	}
	return l, nil
}

// GetLocation returns locationId's details. Authorization: any business
// member.
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

// GetAllLocations lists every location in the caller's business.
// Authorization: any business member.
func (ls *LocationService) GetAllLocations(
	ctx context.Context,
) ([]domain.Location, error) {
	locations, err := ls.repo.GetLocationsByBusinessId(ctx, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get all locations: %w", err)
	}
	return locations, nil
}

// UpdateLocation replaces locationId's name, address, and timezone. There is
// no partial-update path, every field in update is always applied.
// Authorization: admin.
func (ls *LocationService) UpdateLocation(
	ctx context.Context,
	locationId string,
	update domain.LocationUpdate,
) error {
	if err := validateIsAdmin(ctx); err != nil {
		return fmt.Errorf("failed to update location: %w", err)
	}
	if err := domain.ValidateLocationName(update.Name); err != nil {
		return fmt.Errorf("failed to update location: %w", err)
	}
	if update.Address != nil {
		if err := domain.ValidateLocationAddr(*update.Address); err != nil {
			return fmt.Errorf("failed to update location: %w", err)
		}
	}
	if err := domain.ValidateTimezone(update.Timezone); err != nil {
		return fmt.Errorf("failed to update location: %w", err)
	}

	if err := ls.repo.UpdateLocationById(ctx, locationId, update); err != nil {
		return fmt.Errorf("failed to update location: %w", err)
	}
	return nil
}

// DeleteLocation deletes locationId. Authorization: admin.
func (ls *LocationService) DeleteLocation(
	ctx context.Context,
	locationId string,
) error {
	if err := validateIsAdmin(ctx); err != nil {
		return fmt.Errorf("failed to delete location: %w", err)
	}

	if err := ls.repo.DeleteLocation(ctx, locationId); err != nil {
		return fmt.Errorf("failed to delete location: %w", err)
	}
	return nil
}
