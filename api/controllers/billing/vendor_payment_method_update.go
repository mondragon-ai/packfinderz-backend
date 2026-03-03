package billing

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/controllers/vendorcontext"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	"github.com/angelmondragon/packfinderz-backend/internal/paymentmethods"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type vendorPaymentMethodUpdateRequest struct {
	IsDefault *bool `json:"is_default" validate:"required"`
}

// VendorPaymentMethodUpdate toggles the default flag for a stored payment method.
func VendorPaymentMethodUpdate(svc paymentmethods.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "payment method service unavailable"))
			return
		}

		ctx := r.Context()
		storeID, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		var payload vendorPaymentMethodUpdateRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		if payload.IsDefault == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeValidation, "is_default is required"))
			return
		}

		idParam := strings.TrimSpace(chi.URLParam(r, "paymentMethodId"))
		if idParam == "" {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeValidation, "payment method id is required"))
			return
		}

		paymentMethodID, err := uuid.Parse(idParam)
		if err != nil {
			responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid payment method id"))
			return
		}

		method, err := svc.UpdatePaymentMethodDefault(ctx, storeID, paymentMethodID, *payload.IsDefault)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		resp := mapVendorPaymentMethodResponse(method)
		responses.WriteSuccess(w, resp)
	}
}
