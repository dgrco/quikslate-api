package domain

import (
	"context"
	"time"
)

type Role string

const (
	AdminRole    Role = "admin"
	ManagerRole  Role = "manager"
	EmployeeRole Role = "employee"
)

type UserRole struct {
	Id         string    `json:"id"`
	UserId     string    `json:"user_id"`
	BusinessId string    `json:"business_id"`
	LocationId *string   `json:"location_id"`
	Role       Role      `json:"user_role"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type UserRoleRepository interface {
	AssignRole(ctx context.Context, userId, businessId string, locationId *string, role Role) error
	GetUserRole(ctx context.Context, userId, businessId string) (UserRole, error)
	RemoveRole(ctx context.Context, userId, businessId string) error
}
