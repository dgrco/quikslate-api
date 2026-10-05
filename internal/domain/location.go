package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Location is a physical site belonging to a Business, where positions are
// offered and shifts are scheduled. Its timezone drives how that site's
// shift times are interpreted and displayed.

const EmptyLocation = "" // for admins, who are not scoped to a single location

type Location struct {
	Id         string    `json:"id"`
	BusinessId string    `json:"business_id"`
	Name       string    `json:"name"`
	Address    *string   `json:"address"`
	Timezone   string    `json:"timezone"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func ValidateLocationName(locationName string) error {
	if strings.TrimSpace(locationName) == "" {
		return NewValidationError("location name cannot be empty")
	}
	if len(locationName) > MAX_LOCATION_NAME_LEN {
		return NewValidationError(fmt.Sprintf("location name must be less than %d characters", MAX_LOCATION_NAME_LEN))
	}
	return nil
}

func ValidateLocationAddr(addr string) error {
	if len(addr) > MAX_LOCATION_ADDR_LEN {
		return NewValidationError(fmt.Sprintf("location address must be less than %d characters", MAX_LOCATION_ADDR_LEN))
	}
	return nil
}

// ValidateTimezone returns an error unless timezone is an IANA name Go's
// tzdata recognizes (e.g. "America/New_York"). Rejecting here keeps a typo
// from reaching the DB, where the column is a plain TEXT that would happily
// store it and then break every render of that location's schedule.
func ValidateTimezone(timezone string) error {
	if strings.TrimSpace(timezone) == "" {
		return NewValidationError("location timezone cannot be empty")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return NewValidationError(fmt.Sprintf("invalid location timezone: %q", timezone))
	}
	return nil
}

// LocationUpdate is a full replacement of a location's mutable fields:
// there is no partial-update path, so every field is always applied as given.
type LocationUpdate struct {
	Name     string
	Address  *string
	Timezone string
}

type LocationRepository interface {
	CreateLocation(ctx context.Context, businessId, name string, address *string, timezone string) (Location, error)
	GetLocationById(ctx context.Context, id string) (Location, error)
	GetLocationsByBusinessId(ctx context.Context, businessId string) ([]Location, error)
	UpdateLocationById(ctx context.Context, id string, update LocationUpdate) error
	DeleteLocation(ctx context.Context, id string) error
}
