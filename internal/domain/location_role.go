package domain

import (
	"context"
	"time"
)

type LRole string

const (
	Manager   LRole = "manager"
	Employee  LRole = "employee"
	EmptyRole LRole = "" // for admins
)

type LocationRole struct {
	UserId     string    `json:"user_id"`
	BusinessId string    `json:"business_id"`
	LocationId string    `json:"location_id"`
	Role       LRole     `json:"user_role"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type LocationRoleRepository interface {
	AssignRole(ctx context.Context, userId, businessId string, locationId string, role LRole) error
	GetLocationRole(ctx context.Context, userId, locationId string) (LocationRole, error)
	GetLocationRolesByUserAndBusiness(ctx context.Context, userId, businessId string) ([]LocationRole, error)
	RemoveRole(ctx context.Context, userId, locationId string) error
}
