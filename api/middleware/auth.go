package middleware

import (
	"context"
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

func Auth(logg *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := validators.ParseAuthToken(r.Header.Get("Authorization"))
			if err != nil {
				responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeUnauthorized, err, "missing credentials"))
				return
			}

			ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxRole, claims.Role)
			ctx = context.WithValue(ctx, ctxStoreID, claims.StoreID)

			if logg != nil {
				ctx = logg.WithFields(ctx, map[string]any{
					"user_id":    claims.UserID,
					"actor_role": claims.Role,
					"store_id":   claims.StoreID,
				})
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
