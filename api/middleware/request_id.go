package middleware

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

const requestIDHeader = "X-Request-Id"

func RequestID(logg *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get(requestIDHeader)
			if reqID == "" {
				reqID = uuid.NewString()
			}

			w.Header().Set(requestIDHeader, reqID)

			ctx := r.Context()
			if logg != nil {
				ctx = logg.WithRequestID(ctx, reqID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
