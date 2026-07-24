package domain

import (
	"context"
	"fmt"
	"time"
)

type LRole string

const (
	Employee     LRole = "employee"
	Manager      LRole = "manager"			 // manages employees
	LocationLead LRole = "location_lead" // manages (leads) all roles at a location
	EmptyRole    LRole = ""              // for admins/identity-only
)

type LocationRole struct {
	UserId     string    `json:"user_id"`
	BusinessId string    `json:"business_id"`
	LocationId string    `json:"location_id"`
	Role       LRole     `json:"user_role"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func ValidateNonAdminLocationRole(role LRole) error {
	switch role {
	case Manager, Employee, LocationLead:
		return nil
	default:
		return NewValidationError(fmt.Sprintf("invalid non-admin location role: %q", role))
	}
}

func ValidateLocationRole(role LRole) error {
	switch role {
	case Manager, Employee, LocationLead, EmptyRole:
		return nil
	default:
		return NewValidationError(fmt.Sprintf("invalid location role: %q", role))
	}
}

type LocationRoleRepository interface {
	AssignRole(ctx context.Context, userId, businessId string, locationId string, role LRole) error
	GetLocationRole(ctx context.Context, userId, locationId, businessId string) (LocationRole, error)
	GetLocationRolesByUserAndBusiness(ctx context.Context, userId, businessId string) ([]LocationRole, error)
	RemoveRole(ctx context.Context, userId, locationId string) error
	RemoveAllRolesOfUserFromBusiness(ctx context.Context, userId, businessId string) error
}
