package auth

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	"github.com/angelmondragon/packfinderz-backend/internal/auth"
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
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
		responses.WriteSuccess(w, map[string]any{
			"stores": result.Stores,
			"user":   result.User,
		})
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
		responses.WriteSuccess(w, map[string]*users.UserDTO{
			"user": result.User,
		})
	}
}
func AdminAuthRegister(adminRegister auth.AdminRegisterService, svc auth.Service, cfg *config.Config, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.App.IsProd() {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "admin register disabled in production"))
			return
		}
		if adminRegister == nil || svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "admin register unavailable"))
			return
		}

		var body auth.AdminRegisterRequest
		if err := validators.DecodeJSONBody(r, &body); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		if _, err := adminRegister.Register(r.Context(), body); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		result, err := svc.AdminLogin(r.Context(), auth.LoginRequest{
			Email:    body.Email,
			Password: body.Password,
		})
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		w.Header().Set("X-PF-Token", result.AccessToken)
		responses.WriteSuccessStatus(w, http.StatusCreated, map[string]*users.UserDTO{
			"user": result.User,
		})
	}
}
