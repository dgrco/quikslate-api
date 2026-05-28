package domain

import (
	"context"
	"time"
)

type Location struct {
	ID         string    `json:"id"`
	BusinessID string    `json:"business_id"`
	Name       string    `json:"name"`
	Address    string    `json:"address"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Update struct (can be used for partial updates -> simply don't assign a field)
type LocationUpdate struct {
	Name    *string
	Address *string
}

type LocationRepository interface {
	CreateLocation(ctx context.Context, businessID, name, address string) (Location, error)
	GetLocationById(ctx context.Context, id string) (Location, error)
	GetLocationsByBusinessID(ctx context.Context, businessID string) ([]Location, error)
	UpdateLocationById(ctx context.Context, id string, update LocationUpdate) error
	DeleteLocation(ctx context.Context, id string) error
}
