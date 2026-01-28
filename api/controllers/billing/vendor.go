package billing

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/api/controllers/vendorcontext"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	billingsvc "github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type vendorBillingCharge struct {
	ID          string     `json:"id"`
	AmountCents int64      `json:"amount_cents"`
	Currency    string     `json:"currency"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Description *string    `json:"description,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	BilledAt    *time.Time `json:"billed_at,omitempty"`
}

type vendorBillingChargesResponse struct {
	Charges []vendorBillingCharge `json:"charges"`
	Cursor  string                `json:"cursor"`
}

func VendorBillingCharges(svc ChargesService, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if svc == nil {
			responses.WriteError(ctx, logg, w, pkgerrors.New(pkgerrors.CodeInternal, "billing service unavailable"))
			return
		}

		storeID, err := vendorcontext.ResolveVendorStoreID(r)
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		query := r.URL.Query()
		limit, err := parseLimit(query.Get("limit"))
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		var chargeType *enums.ChargeType
		if typ := strings.TrimSpace(query.Get("type")); typ != "" {
			parsed, parseErr := enums.ParseChargeType(typ)
			if parseErr != nil {
				responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, parseErr, "invalid type"))
				return
			}
			parsedType := parsed
			chargeType = &parsedType
		}

		var chargeStatus *enums.ChargeStatus
		if status := strings.TrimSpace(query.Get("status")); status != "" {
			parsed, parseErr := enums.ParseChargeStatus(status)
			if parseErr != nil {
				responses.WriteError(ctx, logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, parseErr, "invalid status"))
				return
			}
			parsedStatus := parsed
			chargeStatus = &parsedStatus
		}

		cursor := strings.TrimSpace(query.Get("cursor"))
		result, err := svc.ListCharges(ctx, billingsvc.ListChargesParams{
			StoreID: storeID,
			Limit:   limit,
			Cursor:  cursor,
			Type:    chargeType,
			Status:  chargeStatus,
		})
		if err != nil {
			responses.WriteError(ctx, logg, w, err)
			return
		}

		payload := vendorBillingChargesResponse{
			Charges: make([]vendorBillingCharge, len(result.Items)),
			Cursor:  result.Cursor,
		}
		for i, charge := range result.Items {
			payload.Charges[i] = toVendorBillingCharge(charge)
		}

		responses.WriteSuccess(w, payload)
	}
}

func parseLimit(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 {
		return 0, pkgerrors.New(pkgerrors.CodeValidation, "limit must be a positive integer")
	}
	return limit, nil
}

func toVendorBillingCharge(c models.Charge) vendorBillingCharge {
	return vendorBillingCharge{
		ID:          c.ID.String(),
		AmountCents: c.AmountCents,
		Currency:    c.Currency,
		Type:        string(c.Type),
		Status:      string(c.Status),
		Description: c.Description,
		CreatedAt:   c.CreatedAt.UTC(),
		BilledAt:    c.BilledAt,
	}
}

type ChargesService interface {
	ListCharges(ctx context.Context, params billingsvc.ListChargesParams) (*billingsvc.ListChargesResult, error)
}
