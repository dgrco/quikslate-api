package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/pkg/auth"
)

const UserIdKey ctxkeys.StringContextKey = "userId"

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

			ctx := context.WithValue(r.Context(), UserIdKey, claims.UserId)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
