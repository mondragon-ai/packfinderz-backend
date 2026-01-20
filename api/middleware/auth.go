package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	pkgAuth "github.com/angelmondragon/packfinderz-backend/pkg/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

// Auth validates a bearer token and seeds the request context with the claims.
func Auth(cfg config.JWTConfig, verifier session.AccessSessionChecker, logg *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := strings.TrimSpace(r.Header.Get("Authorization"))
			if raw == "" {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "missing credentials"))
				return
			}

			token := raw
			if strings.HasPrefix(strings.ToLower(token), "bearer ") {
				token = strings.TrimSpace(token[7:])
			}
			if token == "" {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "missing credentials"))
				return
			}

			claims, err := pkgAuth.ParseAccessToken(cfg, token)
			if err != nil {
				responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeUnauthorized, err, "invalid token"))
				return
			}

			if claims.ID == "" {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "missing session id"))
				return
			}

			if verifier != nil {
				ok, err := verifier.HasSession(r.Context(), claims.ID)
				if err != nil {
					responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "validate session"))
					return
				}
				if !ok {
					responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "session unavailable"))
					return
				}
			}

			ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID.String())
			ctx = context.WithValue(ctx, ctxRole, string(claims.Role))
			if claims.ActiveStoreID != nil {
				ctx = context.WithValue(ctx, ctxStoreID, claims.ActiveStoreID.String())
			}

			if logg != nil {
				fields := map[string]any{
					"user_id":    claims.UserID.String(),
					"actor_role": string(claims.Role),
				}
				if claims.ActiveStoreID != nil {
					fields["store_id"] = claims.ActiveStoreID.String()
				}
				ctx = logg.WithFields(ctx, fields)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
