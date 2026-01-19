package controllers

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type PublicValidateBody struct {
	Name  string `json:"name" validate:"required,min=3,max=64"`
	Email string `json:"email" validate:"required,email"`
}

func PublicValidate(logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body PublicValidateBody
		if err := validators.DecodeJSONBody(r, &body); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		limit, err := validators.ParseQueryInt(r, "limit", 10, 1, 100)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, map[string]any{
			"name":  validators.SanitizeString(body.Name, 64),
			"email": body.Email,
			"limit": limit,
		})
	}
}
