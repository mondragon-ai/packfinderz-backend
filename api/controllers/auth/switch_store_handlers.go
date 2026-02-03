package auth

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	pkgAuth "github.com/angelmondragon/packfinderz-backend/pkg/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"

	"github.com/angelmondragon/packfinderz-backend/internal/auth"
)

type switchStoreRequest struct {
	StoreID      string `json:"store_id" validate:"required,uuid"`
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// AuthSwitchStore mints a new token that targets the requested store.
func AuthSwitchStore(svc auth.SwitchStoreService, cfg config.JWTConfig, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, errors.New(errors.CodeInternal, "switch store service unavailable"))
			return
		}

		var body switchStoreRequest
		if err := validators.DecodeJSONBody(r, &body); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		storeID, err := uuid.Parse(body.StoreID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, errors.Wrap(errors.CodeValidation, err, "invalid store_id"))
			return
		}

		token, err := parseBearerToken(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		claims, err := pkgAuth.ParseAccessToken(cfg, token)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, errors.Wrap(errors.CodeUnauthorized, err, "invalid token"))
			return
		}

		result, err := svc.Switch(r.Context(), auth.SwitchStoreInput{
			UserID:        claims.UserID,
			StoreID:       storeID,
			AccessTokenID: claims.ID,
			RefreshToken:  body.RefreshToken,
		})
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		w.Header().Set("X-PF-Token", result.AccessToken)
		responses.WriteSuccess(w, result)
	}
}
