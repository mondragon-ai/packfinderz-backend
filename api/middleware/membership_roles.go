package middleware

import (
	"context"
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/google/uuid"
)

type MembershipChecker interface {
	UserHasRole(ctx context.Context, userID, storeID uuid.UUID, roles ...enums.MemberRole) (bool, error)
}

// RequireStoreRoles filters requests by store membership roles before executing the handler.
func RequireStoreRoles(checker MembershipChecker, logg *logger.Logger, allowed ...enums.MemberRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if checker == nil {
				responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "membership checker unavailable"))
				return
			}
			if len(allowed) == 0 {
				responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "allowed roles missing"))
				return
			}

			userID := UserIDFromContext(ctx)
			if userID == "" {
				responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
				return
			}

			storeID := StoreIDFromContext(ctx)
			if storeID == "" {
				responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context required"))
				return
			}

			uid, err := uuid.Parse(userID)
			if err != nil {
				responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
				return
			}

			sid, err := uuid.Parse(storeID)
			if err != nil {
				responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
				return
			}

			ok, err := checker.UserHasRole(ctx, uid, sid, allowed...)
			if err != nil {
				responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "check membership role"))
				return
			}
			if !ok {
				responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "insufficient store role"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
