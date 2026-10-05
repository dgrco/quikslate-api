package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/dgrco/quikslate/internal/auth"
	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
)

// Pick the least specific middleware a route's params allow: RequireIdentity
// when there is no {businessId}, RequireBusinessMember when there is no
// {locationId}, and RequireLocationMember only when the route has both.

func parseAccessToken(r *http.Request, jwtSecret string) (*auth.AccessTokenClaims, error) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return nil, domain.ErrUnauthorized
	}

	tokenStr := strings.TrimPrefix(header, "Bearer ")
	claims, err := auth.ValidateAccessToken(tokenStr, jwtSecret)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	return claims, nil
}

func RequireIdentity(authService *service.AuthService, jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := parseAccessToken(r, jwtSecret)
			if err != nil {
				response.WriteError(w, err.Error(), http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxkeys.UserId, claims.UserId)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireBusinessMember(authService *service.AuthService, jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := parseAccessToken(r, jwtSecret)
			if err != nil {
				response.WriteError(w, err.Error(), http.StatusUnauthorized)
				return
			}

			businessId := chi.URLParam(r, "businessId")
			authzCtx, err := authService.GetBusinessMemberAuthzContext(r.Context(), claims.UserId, businessId)
			if err != nil {
				handleServiceError(w, err, "business member authorization verification")
				return
			}

			ctx := context.WithValue(r.Context(), ctxkeys.UserId, claims.UserId)
			ctx = context.WithValue(ctx, ctxkeys.BusinessId, businessId)
			ctx = context.WithValue(ctx, ctxkeys.IsPrimaryAdmin, authzCtx.IsPrimaryAdmin)
			ctx = context.WithValue(ctx, ctxkeys.IsAdmin, authzCtx.IsAdmin)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireLocationMember(authService *service.AuthService, jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := parseAccessToken(r, jwtSecret)
			if err != nil {
				response.WriteError(w, err.Error(), http.StatusUnauthorized)
				return
			}

			businessId := chi.URLParam(r, "businessId")
			locationId := chi.URLParam(r, "locationId")
			authzCtx, err := authService.GetLocationMemberAuthzContext(r.Context(), claims.UserId, businessId, locationId)
			if err != nil {
				handleServiceError(w, err, "location member authorization verification")
				return
			}

			ctx := context.WithValue(r.Context(), ctxkeys.UserId, claims.UserId)
			ctx = context.WithValue(ctx, ctxkeys.BusinessId, businessId)
			ctx = context.WithValue(ctx, ctxkeys.IsPrimaryAdmin, authzCtx.IsPrimaryAdmin)
			ctx = context.WithValue(ctx, ctxkeys.IsAdmin, authzCtx.IsAdmin)
			ctx = context.WithValue(ctx, ctxkeys.LocationId, locationId)
			ctx = context.WithValue(ctx, ctxkeys.Role, authzCtx.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
