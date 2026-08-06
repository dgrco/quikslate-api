package domain

import (
	"context"
	"time"
)

type BusinessMember struct {
	UserId         string    `json:"user_id"`
	BusinessId     string    `json:"business_id"`
	IsPrimaryAdmin bool      `json:"is_primary_admin"`
	IsAdmin        bool      `json:"is_admin"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// BusinessMemberDetail is a BusinessMember joined with its user's name/email.
// It is the shape the member roster actually needs to display.
type BusinessMemberDetail struct {
	UserId         string    `json:"user_id"`
	BusinessId     string    `json:"business_id"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	IsPrimaryAdmin bool      `json:"is_primary_admin"`
	IsAdmin        bool      `json:"is_admin"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type BusinessMemberRepository interface {
	AddUserToBusiness(ctx context.Context, userId, businessId string, isPrimaryAdmin, isAdmin bool) error
	GetBusinessMember(ctx context.Context, userId, businessId string) (BusinessMember, error)
	GetBusinessMemberDetailsByBusinessId(ctx context.Context, businessId string) ([]BusinessMemberDetail, error)
	SetAdminForBusinessMember(ctx context.Context, userId, businessId string, admin bool) error
	GetAdminCount(ctx context.Context, businessId string) (int, error)
	RemoveUserFromBusiness(ctx context.Context, userId, businessId string) error
}
