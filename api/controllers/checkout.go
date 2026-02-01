package controllers

import (
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	checkoutsvc "github.com/angelmondragon/packfinderz-backend/internal/checkout"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"

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

		idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if idempotencyKey == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "Idempotency-Key header required"))
			return
		}

		group, err := svc.Execute(r.Context(), buyerStoreID, payload.CartID, checkoutsvc.CheckoutInput{
			IdempotencyKey:  idempotencyKey,
			ShippingAddress: payload.ShippingAddress,
			PaymentMethod:   payload.PaymentMethod,
			ShippingLine:    payload.ShippingLine,
		})
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccessStatus(w, http.StatusCreated, newCheckoutResponse(group))
	}
}

func buyerStoreIDFromContext(r *http.Request) (uuid.UUID, error) {
	if r == nil {
		return uuid.Nil, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing")
	}
	storeID := middleware.StoreIDFromContext(r.Context())
	if storeID == "" {
		return uuid.Nil, pkgerrors.New(pkgerrors.CodeForbidden, "store context missing")
	}
	buyerStoreID, err := uuid.Parse(storeID)
	if err != nil {
		return uuid.Nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid buyer store id")
	}
	return buyerStoreID, nil
}

type checkoutRequest struct {
	CartID          uuid.UUID           `json:"cart_id" validate:"required,uuid4"`
	ShippingAddress *types.Address      `json:"shipping_address" validate:"required"`
	PaymentMethod   enums.PaymentMethod `json:"payment_method" validate:"required,oneof=cash ach"`
	ShippingLine    *types.ShippingLine `json:"shipping_line,omitempty"`
}

type checkoutResponse struct {
	CheckoutGroupID uuid.UUID              `json:"checkout_group_id"`
	ShippingAddress *types.Address         `json:"shipping_address"`
	PaymentMethod   enums.PaymentMethod    `json:"payment_method"`
	ShippingLine    *types.ShippingLine    `json:"shipping_line,omitempty"`
	VendorOrders    []vendorOrderResponse  `json:"vendor_orders"`
	RejectedVendors []rejectedVendorReport `json:"rejected_vendors,omitempty"`
}

type vendorOrderResponse struct {
	OrderID           uuid.UUID          `json:"order_id"`
	VendorStoreID     uuid.UUID          `json:"vendor_store_id"`
	Status            string             `json:"status"`
	SubtotalCents     int                `json:"subtotal_cents"`
	DiscountsCents    int                `json:"discount_cents"`
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
	VendorStoreID uuid.UUID                 `json:"vendor_store_id"`
	LineItems     []lineItemResponse        `json:"line_items"`
	Warnings      types.VendorGroupWarnings `json:"warnings,omitempty"`
}

func newCheckoutResponse(group *models.CheckoutGroup) checkoutResponse {
	if group == nil {
		return checkoutResponse{}
	}
	vendorOrders := make([]vendorOrderResponse, 0, len(group.VendorOrders))
	rejections := map[uuid.UUID]*rejectedVendorReport{}
	for _, order := range group.VendorOrders {
		items := make([]lineItemResponse, 0, len(order.Items))
		for _, item := range order.Items {
			resp := newLineItemResponse(item)
			items = append(items, resp)
			if item.Status == enums.LineItemStatusRejected {
				report := rejections[order.VendorStoreID]
				if report == nil {
					report = &rejectedVendorReport{VendorStoreID: order.VendorStoreID}
					rejections[order.VendorStoreID] = report
				}
				report.LineItems = append(report.LineItems, resp)
			}
		}
		vendorOrders = append(vendorOrders, vendorOrderResponse{
			OrderID:           order.ID,
			VendorStoreID:     order.VendorStoreID,
			Status:            string(order.Status),
			SubtotalCents:     order.SubtotalCents,
			DiscountsCents:    order.DiscountsCents,
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
	for _, report := range rejections {
		rejected = append(rejected, *report)
	}
	for _, group := range group.CartVendorGroups {
		if group.Status == enums.VendorGroupStatusOK {
			continue
		}
		if _, exists := rejections[group.VendorStoreID]; exists {
			continue
		}
		rejected = append(rejected, rejectedVendorReport{
			VendorStoreID: group.VendorStoreID,
			Warnings:      group.Warnings,
		})
	}
	sort.Slice(rejected, func(i, j int) bool {
		return rejected[i].VendorStoreID.String() < rejected[j].VendorStoreID.String()
	})

	shippingAddress, paymentMethod, shippingLine := collectCheckoutConfirmationFields(group)

	return checkoutResponse{
		CheckoutGroupID: group.ID,
		ShippingAddress: shippingAddress,
		PaymentMethod:   paymentMethod,
		ShippingLine:    shippingLine,
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

func collectCheckoutConfirmationFields(group *models.CheckoutGroup) (*types.Address, enums.PaymentMethod, *types.ShippingLine) {
	var (
		shippingAddress *types.Address
		paymentMethod   enums.PaymentMethod
		shippingLine    *types.ShippingLine
	)
	for _, order := range group.VendorOrders {
		if shippingAddress == nil && order.ShippingAddress != nil {
			shippingAddress = order.ShippingAddress
		}
		if paymentMethod == "" && order.PaymentMethod != "" {
			paymentMethod = order.PaymentMethod
		}
		if shippingLine == nil && order.ShippingLine != nil {
			shippingLine = order.ShippingLine
		}
		if shippingAddress != nil && paymentMethod != "" && shippingLine != nil {
			break
		}
	}
	if paymentMethod == "" {
		paymentMethod = enums.PaymentMethodCash
	}
	return shippingAddress, paymentMethod, shippingLine
}
