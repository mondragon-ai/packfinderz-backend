package controllers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	internalorders "github.com/angelmondragon/packfinderz-backend/internal/orders"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
)

type payoutRepository interface {
	ListPayoutOrders(ctx context.Context, params pagination.Params) (*internalorders.PayoutOrderList, error)
	FindOrderDetail(ctx context.Context, orderID uuid.UUID) (*internalorders.OrderDetail, error)
}

// AdminPayoutOrders returns a paginated list of orders eligible for payout.
func AdminPayoutOrders(repo payoutRepository, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "orders repository unavailable"))
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

		list, err := repo.ListPayoutOrders(r.Context(), params)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "list payout orders"))
			return
		}
		responses.WriteSuccess(w, list)
	}
}

// AdminPayoutOrderDetail returns the expanded detail for a payout-eligible order.
func AdminPayoutOrderDetail(repo payoutRepository, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "orders repository unavailable"))
			return
		}

		rawOrderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
		if rawOrderID == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "order id is required"))
			return
		}
		orderID, err := uuid.Parse(rawOrderID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid order id"))
			return
		}

		detail, err := repo.FindOrderDetail(r.Context(), orderID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeNotFound, "order not found"))
				return
			}
			responses.WriteError(r.Context(), logg, w, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "fetch order detail"))
			return
		}

		if detail.Order == nil || detail.Order.Status != enums.VendorOrderStatusDelivered {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeStateConflict, "order not eligible for payout"))
			return
		}
		if detail.PaymentIntent == nil || detail.PaymentIntent.Status != string(enums.PaymentStatusSettled) {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeStateConflict, "order payment not yet settled"))
			return
		}

		responses.WriteSuccess(w, detail)
	}
}
