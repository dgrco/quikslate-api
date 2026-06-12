package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/auth"
)

func AuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				response.WriteError(w, domain.ErrUnauthorized.Error(), http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(header, "Bearer ")
			claims, err := auth.ValidateJWT(tokenStr, jwtSecret)
			if err != nil {
				response.WriteError(w, domain.ErrUnauthorized.Error(), http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxkeys.UserId, claims.UserId)
			ctx = context.WithValue(ctx, ctxkeys.BusinessId, claims.BusinessId)
			ctx = context.WithValue(ctx, ctxkeys.IsAdmin, claims.IsAdmin)
			ctx = context.WithValue(ctx, ctxkeys.LocationId, claims.LocationId)
			ctx = context.WithValue(ctx, ctxkeys.Role, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
