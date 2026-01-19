package middleware

import (
	"net/http"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

func Logging(logg *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if logg != nil {
				ctx = logg.WithFields(ctx, map[string]any{
					"method": r.Method,
					"path":   r.URL.Path,
				})
			}

			rec := &statusRecorder{ResponseWriter: w}
			start := time.Now()

			if logg != nil {
				logg.Info(ctx, "request.start")
			}

			next.ServeHTTP(rec, r.WithContext(ctx))

			if rec.status == 0 {
				rec.status = http.StatusOK
			}

			if logg != nil {
				ctx = logg.WithFields(ctx, map[string]any{
					"status":      rec.status,
					"duration_ms": time.Since(start).Milliseconds(),
				})
				logg.Info(ctx, "request.complete")
			}
		})
	}
}
