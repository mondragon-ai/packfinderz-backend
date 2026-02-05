package subscriptions

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/controllers/vendorcontext"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	subsvc "github.com/angelmondragon/packfinderz-backend/internal/subscriptions"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type vendorSubscriptionCreateRequest struct {
	SquareCustomerID      string `json:"square_customer_id" validate:"required"`
	SquarePaymentMethodID string `json:"square_payment_method_id" validate:"required"`
	PriceID               string `json:"price_id,omitempty"`
}

type vendorSubscriptionResponse struct {
	ID                   uuid.UUID  `json:"id"`
	SquareSubscriptionID string     `json:"square_subscription_id"`
	Status               string     `json:"status"`
	PriceID              *string    `json:"price_id,omitempty"`
	CurrentPeriodStart   *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd     time.Time  `json:"current_period_end"`
	CancelAtPeriodEnd    bool       `json:"cancel_at_period_end"`
	CanceledAt           *time.Time `json:"canceled_at,omitempty"`
}

func VendorSubscriptionCreate(svc subsvc.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "subscription service unavailable"))
			return
		}

		storeID, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		var payload vendorSubscriptionCreateRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		sub, created, err := svc.Create(r.Context(), storeID, subsvc.CreateSubscriptionInput{
			SquareCustomerID:      payload.SquareCustomerID,
			SquarePaymentMethodID: payload.SquarePaymentMethodID,
			PriceID:               payload.PriceID,
		})
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		resp := newVendorSubscriptionResponse(sub)
		if created {
			responses.WriteSuccessStatus(w, http.StatusCreated, resp)
			return
		}
		responses.WriteSuccess(w, resp)
	}
}

func VendorSubscriptionCancel(svc subsvc.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "subscription service unavailable"))
			return
		}

		storeID, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		if err := svc.Cancel(r.Context(), storeID); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, nil)
	}
}

func VendorSubscriptionPause(svc subsvc.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "subscription service unavailable"))
			return
		}

		storeID, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		if err := svc.Pause(r.Context(), storeID); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, nil)
	}
}

func VendorSubscriptionResume(svc subsvc.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "subscription service unavailable"))
			return
		}

		storeID, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		if err := svc.Resume(r.Context(), storeID); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, nil)
	}
}

func VendorSubscriptionFetch(svc subsvc.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "subscription service unavailable"))
			return
		}

		storeID, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		sub, err := svc.GetActive(r.Context(), storeID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		if sub == nil {
			responses.WriteSuccess(w, nil)
			return
		}
		responses.WriteSuccess(w, newVendorSubscriptionResponse(sub))
	}
}

func newVendorSubscriptionResponse(sub *models.Subscription) *vendorSubscriptionResponse {
	if sub == nil {
		return nil
	}
	return &vendorSubscriptionResponse{
		ID:                   sub.ID,
		SquareSubscriptionID: sub.SquareSubscriptionID,
		Status:               string(sub.Status),
		PriceID:              sub.PriceID,
		CurrentPeriodStart:   sub.CurrentPeriodStart,
		CurrentPeriodEnd:     sub.CurrentPeriodEnd,
		CancelAtPeriodEnd:    sub.CancelAtPeriodEnd,
		CanceledAt:           sub.CanceledAt,
	}
}
