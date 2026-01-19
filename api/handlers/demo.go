package handlers

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

func DemoError(logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		demoErr := errors.New(errors.CodeValidation, "missing demo payload").
			WithDetails(map[string]string{"field": "demo"})

		responses.WriteError(r.Context(), logg, w, demoErr)
	}
}
