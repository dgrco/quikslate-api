package domain

import "context"

// Only use this for business-scoped operations.
// Use LocationMemberAuthzContext for location-scoped operations.
type BusinessMemberAuthzContext struct {
	IsPrimaryAdmin bool `json:"is_primary_admin"`
	IsAdmin        bool `json:"is_admin"`
}

// A "LocationMember" is a combination of BusinessMember and
// LocationRole metadata. Use this for location-scoped operations.
type LocationMemberAuthzContext struct {
	IsPrimaryAdmin bool  `json:"is_primary_admin"`
	IsAdmin        bool  `json:"is_admin"`
	Role           LRole `json:"role"`
}

type AuthzContextRepository interface {
	GetBusinessMemberAuthzContext(ctx context.Context, userId, businessId string) (BusinessMemberAuthzContext, error)
	GetLocationMemberAuthzContext(ctx context.Context, userId, businessId, locationId string) (LocationMemberAuthzContext, error)
}
