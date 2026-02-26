package reviewcontrollers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	"github.com/angelmondragon/packfinderz-backend/internal/reviews"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
)

// CreateReview handles POST /api/v1/reviews.
func CreateReview(svc reviews.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "reviews service unavailable"))
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

		payload := new(reviews.CreateReviewRequest)
		if err := validators.DecodeJSONBody(r, payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		sid, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		uid, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
			return
		}

		input, err := payload.ToInput(sid, uid)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid payload"))
			return
		}

		result, err := svc.CreateReview(r.Context(), input)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, reviews.ReviewToResponse(result))
	}
}

// ListReviews returns the public feed for a vendor store.
func ListReviews(svc reviews.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "reviews service unavailable"))
			return
		}

		storeID := chi.URLParam(r, "storeId")
		if storeID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "store id required"))
			return
		}

		vendorID, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		limit, err := validators.ParseQueryInt(r, "limit", pagination.DefaultLimit, 1, pagination.MaxLimit)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
		params := pagination.Params{
			Limit:  limit,
			Cursor: cursor,
		}

		result, err := svc.ListVisibleReviews(r.Context(), vendorID, params)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, reviews.ReviewListResultToResponse(result))
	}
}

// DeleteReview removes a review when the buyer user created it.
func DeleteReview(svc reviews.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "reviews service unavailable"))
			return
		}

		reviewID := chi.URLParam(r, "reviewId")
		if reviewID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "review id required"))
			return
		}

		id, err := uuid.Parse(reviewID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid review id"))
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

		sid, err := uuid.Parse(storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id"))
			return
		}

		uid, err := uuid.Parse(userID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid user id"))
			return
		}

		if err := svc.DeleteReview(r.Context(), id, sid, uid); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, nil)
	}
}
