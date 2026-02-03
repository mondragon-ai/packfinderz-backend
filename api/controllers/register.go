package controllers

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	"github.com/angelmondragon/packfinderz-backend/internal/auth"
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

// AuthRegister handles onboarding new users with their first store.
func AuthRegister(reg auth.RegisterService, svc auth.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if reg == nil || svc == nil {
			err := pkgerrors.New(pkgerrors.CodeInternal, "auth service unavailable")
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		var body auth.RegisterRequest
		if err := validators.DecodeJSONBody(r, &body); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		if err := reg.Register(r.Context(), body); err != nil {
			if logg != nil {
				logg.Error(r.Context(), "register failed", err)
			}
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		result, err := svc.Login(r.Context(), auth.LoginRequest{Email: body.Email, Password: body.Password})
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
