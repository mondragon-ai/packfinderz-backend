package controllers

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

// StoreProfile returns the active store's profile using the store-scoped JWT.
func StoreProfile(svc stores.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "store service unavailable"))
			return
		}

		storeID := middleware.StoreIDFromContext(r.Context())
		if storeID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
			return
		}

		id, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		profile, err := svc.GetByID(r.Context(), id)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, profile)
	}
}

// StoreUpdateRequest contains the payload for updating store fields.
type storeUpdateRequest struct {
	CompanyName *string         `json:"company_name,omitempty" validate:"omitempty,min=1"`
	Description *string         `json:"description,omitempty"`
	Phone       *string         `json:"phone,omitempty"`
	Email       *string         `json:"email,omitempty" validate:"omitempty,email"`
	Social      *types.Social   `json:"social,omitempty"`
	BannerURL   *string         `json:"banner_url,omitempty"`
	LogoURL     *string         `json:"logo_url,omitempty"`
	Ratings     *map[string]int `json:"ratings,omitempty"`
	Categories  *[]string       `json:"categories,omitempty"`
}

func (r storeUpdateRequest) toInput() stores.UpdateStoreInput {
	return stores.UpdateStoreInput{
		CompanyName: r.CompanyName,
		Description: r.Description,
		Phone:       r.Phone,
		Email:       r.Email,
		Social:      r.Social,
		BannerURL:   r.BannerURL,
		LogoURL:     r.LogoURL,
		Ratings:     r.Ratings,
		Categories:  r.Categories,
	}
}

// StoreUpdate adjusts the allowed mutable fields for the active store.
func StoreUpdate(svc stores.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "store service unavailable"))
			return
		}

		storeID := middleware.StoreIDFromContext(r.Context())
		if storeID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
			return
		}

		userID := middleware.UserIDFromContext(r.Context())
		if userID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
			return
		}

		uid, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
			return
		}

		sid, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		var payload storeUpdateRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		profile, err := svc.Update(r.Context(), uid, sid, payload.toInput())
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, profile)
	}
}

// StoreUsers returns the membership roster for managers/owners.
func StoreUsers(svc stores.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "store service unavailable"))
			return
		}

		storeID := middleware.StoreIDFromContext(r.Context())
		if storeID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
			return
		}

		userID := middleware.UserIDFromContext(r.Context())
		if userID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeUnauthorized, "user context missing"))
			return
		}

		uid, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
			return
		}

		sid, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		users, err := svc.ListUsers(r.Context(), uid, sid)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, users)
	}
}
