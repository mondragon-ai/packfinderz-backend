package middleware

import (
	"fmt"
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

func Recoverer(logg *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					err := fmt.Errorf("panic: %v", rec)
					ctx := r.Context()
					if logg != nil {
						ctx = logg.WithFields(ctx, map[string]any{"panic": rec})
						logg.Error(ctx, "panic.recovered", err)
					}
					responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeInternal, err, "panic"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
