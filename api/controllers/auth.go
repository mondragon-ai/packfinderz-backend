package controllers

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	"github.com/angelmondragon/packfinderz-backend/internal/auth"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

// AuthLogin wires the login endpoint into the HTTP layer.
func AuthLogin(svc auth.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			err := pkgerrors.New(pkgerrors.CodeInternal, "auth service unavailable")
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		var body auth.LoginRequest
		if err := validators.DecodeJSONBody(r, &body); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		result, err := svc.Login(r.Context(), body)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		w.Header().Set("X-PF-Token", result.AccessToken)
		responses.WriteSuccess(w, result)
	}
}

func AdminAuthLogin(svc auth.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			err := pkgerrors.New(pkgerrors.CodeInternal, "auth service unavailable")
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		var body auth.LoginRequest
		if err := validators.DecodeJSONBody(r, &body); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		result, err := svc.AdminLogin(r.Context(), body)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		w.Header().Set("X-PF-Token", result.AccessToken)
		responses.WriteSuccess(w, result)
	}
}
