package middleware

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

func StoreContext(logg *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if StoreIDFromContext(r.Context()) == "" {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
