package billing

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/api/controllers/vendorcontext"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	"github.com/angelmondragon/packfinderz-backend/internal/paymentmethods"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/google/uuid"
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

type vendorPaymentMethodsResponse struct {
	PaymentMethods []vendorPaymentMethodResponse `json:"payment_methods"`
}

// PaymentMethodsService describes the billing helper used by list handlers.
type PaymentMethodsService interface {
	ListPaymentMethods(ctx context.Context, storeID uuid.UUID) ([]models.PaymentMethod, error)
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

		resp := mapVendorPaymentMethodResponse(method)
		responses.WriteSuccessStatus(w, http.StatusCreated, resp)
	}
}

// VendorPaymentMethodsList returns the stored cards for the authenticated vendor.
func VendorPaymentMethodsList(svc PaymentMethodsService, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "payment methods service unavailable"))
			return
		}

		storeID, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		methods, err := svc.ListPaymentMethods(ctx, storeID)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		resp := vendorPaymentMethodsResponse{
			PaymentMethods: make([]vendorPaymentMethodResponse, 0, len(methods)),
		}
		for _, method := range methods {
			// take the address to share the pointer data safely
			tmp := method
			resp.PaymentMethods = append(resp.PaymentMethods, mapVendorPaymentMethodResponse(&tmp))
		}

		responses.WriteSuccess(w, resp)
	}
}

func mapVendorPaymentMethodResponse(method *models.PaymentMethod) vendorPaymentMethodResponse {
	if method == nil {
		return vendorPaymentMethodResponse{}
	}
	return vendorPaymentMethodResponse{
		ID:        method.ID.String(),
		Brand:     method.CardBrand,
		Last4:     method.CardLast4,
		ExpMonth:  method.CardExpMonth,
		ExpYear:   method.CardExpYear,
		IsDefault: method.IsDefault,
		CreatedAt: method.CreatedAt.UTC(),
	}
}
