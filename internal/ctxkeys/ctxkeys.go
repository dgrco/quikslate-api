package ctxkeys

import (
	"context"

	"github.com/dgrco/quikslate/internal/domain"
)

type contextKey string

const (
	UserId     contextKey = "userId"
	BusinessId contextKey = "businessId"
	LocationId contextKey = "locationId"
	IsAdmin    contextKey = "isAdmin"
	Role       contextKey = "role"
)

func GetUserId(ctx context.Context) string {
	v, _ := ctx.Value(UserId).(string)
	return v
}

func GetBusinessId(ctx context.Context) string {
	v, _ := ctx.Value(BusinessId).(string)
	return v
}

func GetLocationId(ctx context.Context) string {
	v, _ := ctx.Value(LocationId).(string)
	return v
}

func GetIsAdmin(ctx context.Context) bool {
	v, _ := ctx.Value(IsAdmin).(bool)
	return v
}

func GetRole(ctx context.Context) domain.LRole {
	v, _ := ctx.Value(Role).(domain.LRole)
	return v
}
