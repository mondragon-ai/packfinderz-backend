package billing

import (
	"net/http"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/api/controllers/vendorcontext"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	"github.com/angelmondragon/packfinderz-backend/internal/paymentmethods"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type vendorPaymentMethodCreateRequest struct {
	SourceID          string `json:"source_id" validate:"required"`
	CardholderName    string `json:"cardholder_name,omitempty"`
	VerificationToken string `json:"verification_token,omitempty"`
	IsDefault         bool   `json:"is_default,omitempty"`
}

type vendorPaymentMethodResponse struct {
	ID        string    `json:"id"`
	Brand     *string   `json:"card_brand,omitempty"`
	Last4     *string   `json:"card_last4,omitempty"`
	ExpMonth  *int      `json:"card_exp_month,omitempty"`
	ExpYear   *int      `json:"card_exp_year,omitempty"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
}

// VendorPaymentMethodCreate handles card-on-file registration.
func VendorPaymentMethodCreate(svc paymentmethods.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "payment method service unavailable"))
			return
		}

		storeID, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		var payload vendorPaymentMethodCreateRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))

		method, err := svc.StoreCard(r.Context(), storeID, paymentmethods.StoreCardInput{
			SourceID:          payload.SourceID,
			CardholderName:    payload.CardholderName,
			VerificationToken: payload.VerificationToken,
			IsDefault:         payload.IsDefault,
			IdempotencyKey:    idempotencyKey,
		})
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		resp := vendorPaymentMethodResponse{
			ID:        method.ID.String(),
			Brand:     method.CardBrand,
			Last4:     method.CardLast4,
			ExpMonth:  method.CardExpMonth,
			ExpYear:   method.CardExpYear,
			IsDefault: method.IsDefault,
			CreatedAt: method.CreatedAt.UTC(),
		}
		responses.WriteSuccessStatus(w, http.StatusCreated, resp)
	}
}
