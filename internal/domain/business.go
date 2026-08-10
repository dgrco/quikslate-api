package domain

import (
	"context"
	"strings"
	"time"
)

// Business is the top-level tenant: every location, position, and shift
// belongs to one. This file covers the Business entity itself; membership is
// in business_member.go.

type Business struct {
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ValidateBusinessName returns an error if the businessName is empty
func ValidateBusinessName(businessName string) error {
	if strings.TrimSpace(businessName) == "" {
		return NewValidationError("business name cannot be empty")
	}
	return nil
}

type BusinessRepository interface {
	CreateBusiness(ctx context.Context, name string) (Business, error)
	GetBusinessById(ctx context.Context, id string) (Business, error)
	GetBusinessesByUserId(ctx context.Context, userId string) ([]Business, error)
	ChangeBusinessName(ctx context.Context, id, newName string) error
	DeleteBusiness(ctx context.Context, id string) error
}
