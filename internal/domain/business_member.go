package domain

import (
	"context"
	"time"
)

type BusinessMember struct {
	UserId           string     `json:"user_id"`
	BusinessId       string     `json:"business_id"`
	IsPrimaryAdmin   bool				`json:"is_primary_admin"`
	IsAdmin          bool       `json:"is_admin"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type BusinessMemberRepository interface {
	AddUserToBusiness(ctx context.Context, userId, businessId string, isPrimaryAdmin, isAdmin bool) error
	GetBusinessMember(ctx context.Context, userId, businessId string) (BusinessMember, error)
	GetBusinessMembersByUserId(ctx context.Context, userId string) ([]BusinessMember, error)
	SetAdminForBusinessMember(ctx context.Context, userId, businessId string, admin bool) error
	GetAdminCount(ctx context.Context, businessId string) (int, error)
	RemoveUserFromBusiness(ctx context.Context, userId, businessId string) error
}
