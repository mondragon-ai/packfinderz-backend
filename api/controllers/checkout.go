package controllers

import (
	"net/http"
	"sort"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	checkoutsvc "github.com/angelmondragon/packfinderz-backend/internal/checkout"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

// Checkout handles submission of the buyer store's active cart.
func Checkout(svc checkoutsvc.Service, storeSvc stores.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "checkout service unavailable"))
			return
		}
		if storeSvc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "store service unavailable"))
			return
		}

		buyerStoreID, err := buyerStoreIDFromContext(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		store, err := storeSvc.GetByID(r.Context(), buyerStoreID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}
		if store.Type != enums.StoreTypeBuyer {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "buyer store required for checkout"))
			return
		}

		var payload checkoutRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		group, err := svc.Execute(r.Context(), buyerStoreID, payload.CartID, checkoutsvc.CheckoutInput{
			AttributedAdClickID: payload.AttributedAdClickID,
		})
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccessStatus(w, http.StatusCreated, newCheckoutResponse(group))
	}
}

type checkoutRequest struct {
	CartID              uuid.UUID  `json:"cart_id" validate:"required,uuid4"`
	AttributedAdClickID *uuid.UUID `json:"attributed_ad_click_id,omitempty" validate:"omitempty,uuid4"`
}

type checkoutResponse struct {
	CheckoutGroupID uuid.UUID              `json:"checkout_group_id"`
	VendorOrders    []vendorOrderResponse  `json:"vendor_orders"`
	RejectedVendors []rejectedVendorReport `json:"rejected_vendors,omitempty"`
}

type vendorOrderResponse struct {
	OrderID           uuid.UUID          `json:"order_id"`
	VendorStoreID     uuid.UUID          `json:"vendor_store_id"`
	Status            string             `json:"status"`
	SubtotalCents     int                `json:"subtotal_cents"`
	DiscountCents     int                `json:"discount_cents"`
	TaxCents          int                `json:"tax_cents"`
	TransportFeeCents int                `json:"transport_fee_cents"`
	TotalCents        int                `json:"total_cents"`
	BalanceDueCents   int                `json:"balance_due_cents"`
	Items             []lineItemResponse `json:"items"`
}

type lineItemResponse struct {
	LineItemID     uuid.UUID  `json:"line_item_id"`
	ProductID      *uuid.UUID `json:"product_id,omitempty"`
	ProductName    string     `json:"product_name"`
	Qty            int        `json:"qty"`
	Unit           string     `json:"unit"`
	UnitPriceCents int        `json:"unit_price_cents"`
	DiscountCents  int        `json:"discount_cents"`
	TotalCents     int        `json:"total_cents"`
	Status         string     `json:"status"`
	Notes          *string    `json:"notes,omitempty"`
}

type rejectedVendorReport struct {
	VendorStoreID uuid.UUID          `json:"vendor_store_id"`
	LineItems     []lineItemResponse `json:"line_items"`
}

func newCheckoutResponse(group *models.CheckoutGroup) checkoutResponse {
	if group == nil {
		return checkoutResponse{}
	}
	vendorOrders := make([]vendorOrderResponse, 0, len(group.VendorOrders))
	rejections := map[uuid.UUID][]lineItemResponse{}
	for _, order := range group.VendorOrders {
		items := make([]lineItemResponse, 0, len(order.Items))
		for _, item := range order.Items {
			resp := newLineItemResponse(item)
			items = append(items, resp)
			if item.Status == enums.LineItemStatusRejected {
				rejections[order.VendorStoreID] = append(rejections[order.VendorStoreID], resp)
			}
		}
		vendorOrders = append(vendorOrders, vendorOrderResponse{
			OrderID:           order.ID,
			VendorStoreID:     order.VendorStoreID,
			Status:            string(order.Status),
			SubtotalCents:     order.SubtotalCents,
			DiscountCents:     order.DiscountCents,
			TaxCents:          order.TaxCents,
			TransportFeeCents: order.TransportFeeCents,
			TotalCents:        order.TotalCents,
			BalanceDueCents:   order.BalanceDueCents,
			Items:             items,
		})
	}

	sort.Slice(vendorOrders, func(i, j int) bool {
		return vendorOrders[i].VendorStoreID.String() < vendorOrders[j].VendorStoreID.String()
	})

	rejected := make([]rejectedVendorReport, 0, len(rejections))
	for vendorID, items := range rejections {
		rejected = append(rejected, rejectedVendorReport{
			VendorStoreID: vendorID,
			LineItems:     items,
		})
	}
	sort.Slice(rejected, func(i, j int) bool {
		return rejected[i].VendorStoreID.String() < rejected[j].VendorStoreID.String()
	})

	return checkoutResponse{
		CheckoutGroupID: group.ID,
		VendorOrders:    vendorOrders,
		RejectedVendors: rejected,
	}
}

func newLineItemResponse(item models.OrderLineItem) lineItemResponse {
	productName := item.Name
	if productName == "" && item.ProductID != nil {
		productName = item.ProductID.String()
	}
	return lineItemResponse{
		LineItemID:     item.ID,
		ProductID:      item.ProductID,
		ProductName:    productName,
		Qty:            item.Qty,
		Unit:           string(item.Unit),
		UnitPriceCents: item.UnitPriceCents,
		DiscountCents:  item.DiscountCents,
		TotalCents:     item.TotalCents,
		Status:         string(item.Status),
		Notes:          item.Notes,
	}
}
