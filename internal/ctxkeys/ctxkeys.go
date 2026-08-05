package ctxkeys

import (
	"context"

	"github.com/dgrco/quikslate/internal/domain"
)

type contextKey string

const (
	UserId         contextKey = "userId"
	BusinessId     contextKey = "businessId"
	LocationId     contextKey = "locationId"
	IsPrimaryAdmin contextKey = "isPrimaryAdmin"
	IsAdmin        contextKey = "isAdmin"
	Role           contextKey = "role"
)

// GetUserId returns the caller's user ID, as set by RequireIdentity. Returns
// "" if unset (e.g. called outside any of the auth middlewares).
func GetUserId(ctx context.Context) string {
	v, _ := ctx.Value(UserId).(string)
	return v
}

// GetBusinessId returns the caller's business ID, as set by
// RequireBusinessMember or RequireLocationMember. Returns "" if unset.
func GetBusinessId(ctx context.Context) string {
	v, _ := ctx.Value(BusinessId).(string)
	return v
}

// GetLocationId returns the caller's location ID, as set by
// RequireLocationMember. Returns "" if unset (e.g. under a
// business-scoped-only route).
func GetLocationId(ctx context.Context) string {
	v, _ := ctx.Value(LocationId).(string)
	return v
}

// GetIsAdmin reports whether the caller is a business admin, as set by
// RequireBusinessMember or RequireLocationMember. Returns false if unset.
func GetIsAdmin(ctx context.Context) bool {
	v, _ := ctx.Value(IsAdmin).(bool)
	return v
}

// GetIsPrimaryAdmin reports whether the caller is the primary (undemotable)
// admin, as set by RequireBusinessMember or RequireLocationMember. Returns
// false if unset.
func GetIsPrimaryAdmin(ctx context.Context) bool {
	v, _ := ctx.Value(IsPrimaryAdmin).(bool)
	return v
}

// GetRole returns the caller's role at their session location, as set by
// RequireLocationMember. Returns domain.EmptyRole if unset — which is
// ambiguous between "not under RequireLocationMember at all" and "a
// non-admin genuinely has no role here"; see resolveCallerRole in
// internal/service/authorization_checks.go for the caller-side disambiguation.
func GetRole(ctx context.Context) domain.LRole {
	v, _ := ctx.Value(Role).(domain.LRole)
	return v
}
