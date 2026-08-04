package domain

import (
	"context"
	"strings"
	"time"
)

const EmptyLocation = "" // for admins

type Location struct {
	Id         string    `json:"id"`
	BusinessId string    `json:"business_id"`
	Name       string    `json:"name"`
	Address    *string   `json:"address"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ValidateLocationName returns an error if the locationName is empty
func ValidateLocationName(locationName string) error {
	if strings.TrimSpace(locationName) == "" {
		return NewValidationError("location name cannot be empty")
	}
	return nil
}

// LocationUpdate is a full replacement of a location's mutable fields —
// there is no partial-update path, so both fields are always applied as given.
type LocationUpdate struct {
	Name    string
	Address *string
}

type LocationRepository interface {
	CreateLocation(ctx context.Context, businessId, name string, address *string) (Location, error)
	GetLocationById(ctx context.Context, id string) (Location, error)
	GetLocationsByBusinessId(ctx context.Context, businessId string) ([]Location, error)
	UpdateLocationById(ctx context.Context, id string, update LocationUpdate) error
	DeleteLocation(ctx context.Context, id string) error
}
