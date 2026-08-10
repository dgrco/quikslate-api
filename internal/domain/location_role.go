package domain

import (
	"context"
	"fmt"
	"time"
)

// LocationRole is the role a business member holds at one specific
// location. It is what internal/service/authorization_checks.go checks for
// location-scoped actions; business admins bypass it entirely, see the
// Admin constant below.

type LRole string

const (
	Employee     LRole = "employee"
	Manager      LRole = "manager"       // manages employees
	LocationLead LRole = "location_lead" // manages (leads) all roles at a location
	EmptyRole    LRole = ""              // no location-scoped role (identity-only sessions)

	// Admin is a synthetic, in-memory-only role: business admins never get a
	// real location_roles row (the DB column is a NOT NULL enum that doesn't
	// even include this value), so this exists purely to give canActOnRole an
	// unambiguous "top of the hierarchy" value distinct from EmptyRole, which
	// also legitimately means "this non-admin has no role at this location."
	Admin LRole = "admin"
)

type LocationRole struct {
	UserId     string    `json:"user_id"`
	BusinessId string    `json:"business_id"`
	LocationId string    `json:"location_id"`
	Role       LRole     `json:"user_role"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// LocationRoleDetail is a LocationRole joined with its user's name/email,
// the shape a location's roster actually needs to display.
type LocationRoleDetail struct {
	UserId     string    `json:"user_id"`
	BusinessId string    `json:"business_id"`
	LocationId string    `json:"location_id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Role       LRole     `json:"role"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ValidateNonAdminLocationRole returns an error unless role is one of the
// three real location_roles enum values (Manager, Employee, LocationLead).
// Use this where EmptyRole and Admin (which have no location_roles row)
// would be meaningless, e.g. when assigning someone a role at a location.
func ValidateNonAdminLocationRole(role LRole) error {
	switch role {
	case Manager, Employee, LocationLead:
		return nil
	default:
		return NewValidationError(fmt.Sprintf("invalid non-admin location role: %q", role))
	}
}

// ValidateLocationRole returns an error unless role is a real location_roles
// enum value or EmptyRole. Unlike ValidateNonAdminLocationRole, it accepts
// EmptyRole, use this where "no role at this location" is a legitimate
// value, not just an absence.
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
	GetLocationRolesByLocationId(ctx context.Context, locationId string) ([]LocationRoleDetail, error)
	RemoveRole(ctx context.Context, userId, locationId string) error
	RemoveAllRolesOfUserFromBusiness(ctx context.Context, userId, businessId string) error
}
