package domain

import "context"

// The two authorization-context shapes internal/service/authorization_checks.go
// checks against. Don't use the business one where a location role
// actually matters.

// BusinessMemberAuthzContext holds the fields needed to authorize a
// business-scoped operation. Use LocationMemberAuthzContext for
// location-scoped operations instead.
type BusinessMemberAuthzContext struct {
	IsPrimaryAdmin bool `json:"is_primary_admin"`
	IsAdmin        bool `json:"is_admin"`
}

// LocationMemberAuthzContext combines BusinessMemberAuthzContext with the
// caller's LocationRole. Use this for location-scoped operations.
type LocationMemberAuthzContext struct {
	IsPrimaryAdmin bool  `json:"is_primary_admin"`
	IsAdmin        bool  `json:"is_admin"`
	Role           LRole `json:"role"`
}

type AuthzContextRepository interface {
	GetBusinessMemberAuthzContext(ctx context.Context, userId, businessId string) (BusinessMemberAuthzContext, error)
	GetLocationMemberAuthzContext(ctx context.Context, userId, businessId, locationId string) (LocationMemberAuthzContext, error)
}
