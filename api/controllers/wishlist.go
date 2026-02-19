package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/internal/wishlist"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type addWishlistItemPayload struct {
	ProductID string `json:"product_id"`
}

// WishlistList returns the paginated wishlist for the active store.
func WishlistList(svc wishlist.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "wishlist service unavailable"))
			return
		}

		storeID := middleware.StoreIDFromContext(ctx)
		if storeID == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
			return
		}

		sid, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		limit := 0
		if limitStr := strings.TrimSpace(r.URL.Query().Get("limit")); limitStr != "" {
			value, err := strconv.Atoi(limitStr)
			if err != nil || value <= 0 {
				responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeValidation, "limit must be a positive integer"))
				return
			}
			limit = value
		}

		cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
		resp, err := svc.GetWishlist(ctx, sid, cursor, limit)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		responses.WriteSuccess(w, resp)
	}
}

// WishlistIDs returns only product IDs liked by the store.
func WishlistIDs(svc wishlist.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "wishlist service unavailable"))
			return
		}

		storeID := middleware.StoreIDFromContext(ctx)
		if storeID == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
			return
		}

		sid, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		limit := 0
		if limitStr := strings.TrimSpace(r.URL.Query().Get("limit")); limitStr != "" {
			value, err := strconv.Atoi(limitStr)
			if err != nil || value <= 0 {
				responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeValidation, "limit must be a positive integer"))
				return
			}
			limit = value
		}

		cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
		resp, err := svc.GetWishlistIDs(ctx, sid, cursor, limit)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}
		responses.WriteSuccess(w, resp)
	}
}

// WishlistAddItem adds a product to the store's wishlist.
func WishlistAddItem(svc wishlist.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "wishlist service unavailable"))
			return
		}

		storeID := middleware.StoreIDFromContext(ctx)
		if storeID == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
			return
		}

		sid, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		var payload addWishlistItemPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid payload"))
			return
		}

		raw := strings.TrimSpace(payload.ProductID)
		if raw == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeValidation, "product id is required"))
			return
		}

		productID, err := uuid.Parse(raw)
		if err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid product id"))
			return
		}

		if err := svc.AddItem(ctx, sid, productID); err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}
		responses.WriteSuccess(w, map[string]bool{"added": true})
	}
}

// WishlistRemoveItem removes a liked product from the store's wishlist.
func WishlistRemoveItem(svc wishlist.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "wishlist service unavailable"))
			return
		}

		storeID := middleware.StoreIDFromContext(ctx)
		if storeID == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing"))
			return
		}

		sid, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		raw := strings.TrimSpace(chi.URLParam(r, "productId"))
		if raw == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeValidation, "product id is required"))
			return
		}

		productID, err := uuid.Parse(raw)
		if err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid product id"))
			return
		}

		if err := svc.RemoveItem(ctx, sid, productID); err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}
		responses.WriteSuccess(w, map[string]bool{"removed": true})
	}
}
