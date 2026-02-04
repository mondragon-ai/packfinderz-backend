package controllers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/internal/checkout"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
)

// CheckoutConfirmation returns the latest state for a checkout group or cart.
func CheckoutConfirmation(repo checkout.Repository, storeSvc stores.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "checkout repository unavailable"))
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
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "buyer store required"))
			return
		}

		identifier := chi.URLParam(r, "identifier")
		if identifier == "" {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "identifier required"))
			return
		}

		id, err := uuid.Parse(identifier)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeValidation, "invalid checkout identifier"))
			return
		}

		group, err := repo.FindByCheckoutGroupID(r.Context(), id)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}
		if group == nil {
			group, err = repo.FindByCartID(r.Context(), id)
			if err != nil {
				responses.WriteError(r.Context(), logg, w, err)
				return
			}
		}
		if group == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeNotFound, "checkout not found"))
			return
		}
		if group.BuyerStoreID != buyerStoreID {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeForbidden, "checkout does not belong to this store"))
			return
		}

		responses.WriteSuccessStatus(w, http.StatusOK, newCheckoutConfirmationResponse(group))
	}
}

type checkoutConfirmationResponse struct {
	CheckoutGroupID uuid.UUID                 `json:"checkout_group_id"`
	CartID          *uuid.UUID                `json:"cart_id,omitempty"`
	VendorOrders    []vendorOrderConfirmation `json:"vendor_orders"`
}

type vendorOrderConfirmation struct {
	OrderID           uuid.UUID                 `json:"order_id"`
	VendorStoreID     uuid.UUID                 `json:"vendor_store_id"`
	Status            string                    `json:"status"`
	PaymentStatus     string                    `json:"payment_status"`
	PaymentMethod     string                    `json:"payment_method"`
	AmountCents       int                       `json:"amount_cents"`
	BalanceDueCents   int                       `json:"balance_due_cents"`
	FulfillmentStatus string                    `json:"fulfillment_status"`
	ShippingStatus    string                    `json:"shipping_status"`
	PaymentIntent     *paymentIntentSummary     `json:"payment_intent,omitempty"`
	Assignment        *assignmentSummary        `json:"active_assignment,omitempty"`
	SubtotalCents     int                       `json:"subtotal_cents"`
	TotalCents        int                       `json:"total_cents"`
	Warnings          types.VendorGroupWarnings `json:"warnings,omitempty"`
	Promo             *types.VendorGroupPromo   `json:"promo,omitempty"`
	Items             []lineItemConfirmation    `json:"items"`
}

type paymentIntentSummary struct {
	ID              uuid.UUID  `json:"id"`
	Method          string     `json:"method"`
	Status          string     `json:"status"`
	AmountCents     int        `json:"amount_cents"`
	CashCollectedAt *time.Time `json:"cash_collected_at,omitempty"`
	VendorPaidAt    *time.Time `json:"vendor_paid_at,omitempty"`
}

type assignmentSummary struct {
	ID                      uuid.UUID  `json:"id"`
	AgentUserID             uuid.UUID  `json:"agent_user_id"`
	AssignedByUserID        *uuid.UUID `json:"assigned_by_user_id,omitempty"`
	AssignedAt              time.Time  `json:"assigned_at"`
	UnassignedAt            *time.Time `json:"unassigned_at,omitempty"`
	PickupTime              *time.Time `json:"pickup_time,omitempty"`
	DeliveryTime            *time.Time `json:"delivery_time,omitempty"`
	CashPickupTime          *time.Time `json:"cash_pickup_time,omitempty"`
	PickupSignatureGCSKey   *string    `json:"pickup_signature_gcs_key,omitempty"`
	DeliverySignatureGCSKey *string    `json:"delivery_signature_gcs_key,omitempty"`
}

type lineItemConfirmation struct {
	LineItemID     uuid.UUID              `json:"line_item_id"`
	ProductID      *uuid.UUID             `json:"product_id,omitempty"`
	ProductName    string                 `json:"product_name"`
	Category       string                 `json:"category"`
	Strain         *string                `json:"strain,omitempty"`
	Classification *string                `json:"classification,omitempty"`
	Unit           string                 `json:"unit"`
	Qty            int                    `json:"qty"`
	UnitPriceCents int                    `json:"unit_price_cents"`
	DiscountCents  int                    `json:"discount_cents"`
	TotalCents     int                    `json:"total_cents"`
	Status         string                 `json:"status"`
	Notes          *string                `json:"notes,omitempty"`
	Warnings       types.CartItemWarnings `json:"warnings,omitempty"`
}

func buildLineItemConfirmations(items []models.OrderLineItem) []lineItemConfirmation {
	out := make([]lineItemConfirmation, 0, len(items))
	for _, item := range items {
		out = append(out, lineItemConfirmation{
			LineItemID:     item.ID,
			ProductID:      item.ProductID,
			ProductName:    item.Name,
			Category:       item.Category,
			Strain:         item.Strain,
			Classification: item.Classification,
			Unit:           string(item.Unit),
			Qty:            item.Qty,
			UnitPriceCents: item.UnitPriceCents,
			DiscountCents:  item.DiscountCents,
			TotalCents:     item.TotalCents,
			Status:         string(item.Status),
			Notes:          item.Notes,
			Warnings:       item.Warnings,
		})
	}
	return out
}

type cartVendorGroupSummary struct {
	VendorStoreID uuid.UUID                 `json:"vendor_store_id"`
	Status        string                    `json:"status"`
	SubtotalCents int                       `json:"subtotal_cents"`
	TotalCents    int                       `json:"total_cents"`
	Warnings      types.VendorGroupWarnings `json:"warnings,omitempty"`
	Promo         *types.VendorGroupPromo   `json:"promo,omitempty"`
}

func newCheckoutConfirmationResponse(group *models.CheckoutGroup) checkoutConfirmationResponse {
	if group == nil {
		return checkoutConfirmationResponse{}
	}

	vendorGroupByVendor := map[uuid.UUID]models.CartVendorGroup{}
	for _, groupRow := range group.CartVendorGroups {
		vendorGroupByVendor[groupRow.VendorStoreID] = groupRow
	}

	vendorOrders := make([]vendorOrderConfirmation, 0, len(group.VendorOrders))
	for _, order := range group.VendorOrders {
		var groupRow *models.CartVendorGroup
		if row, ok := vendorGroupByVendor[order.VendorStoreID]; ok {
			tmp := row
			groupRow = &tmp
		}
		vendorOrders = append(vendorOrders, newVendorOrderConfirmation(order, groupRow))
	}

	vendorGroups := make([]cartVendorGroupSummary, 0, len(group.CartVendorGroups))
	for _, groupRow := range group.CartVendorGroups {
		vendorGroups = append(vendorGroups, cartVendorGroupSummary{
			VendorStoreID: groupRow.VendorStoreID,
			Status:        string(groupRow.Status),
			SubtotalCents: groupRow.SubtotalCents,
			TotalCents:    groupRow.TotalCents,
			Warnings:      groupRow.Warnings,
			Promo:         groupRow.Promo,
		})
	}
	_ = vendorGroups

	return checkoutConfirmationResponse{
		CheckoutGroupID: group.ID,
		CartID:          group.CartID,
		VendorOrders:    vendorOrders,
	}
}

func newVendorOrderConfirmation(order models.VendorOrder, groupRow *models.CartVendorGroup) vendorOrderConfirmation {
	var intent *paymentIntentSummary
	if order.PaymentIntent != nil {
		intent = &paymentIntentSummary{
			ID:              order.PaymentIntent.ID,
			Method:          string(order.PaymentIntent.Method),
			Status:          string(order.PaymentIntent.Status),
			AmountCents:     order.PaymentIntent.AmountCents,
			CashCollectedAt: order.PaymentIntent.CashCollectedAt,
			VendorPaidAt:    order.PaymentIntent.VendorPaidAt,
		}
	}

	return vendorOrderConfirmation{
		OrderID:           order.ID,
		VendorStoreID:     order.VendorStoreID,
		Status:            string(order.Status),
		PaymentStatus:     stringOrEmpty(order.PaymentIntent),
		PaymentMethod:     string(order.PaymentMethod),
		AmountCents:       order.TotalCents,
		BalanceDueCents:   order.BalanceDueCents,
		FulfillmentStatus: string(order.FulfillmentStatus),
		ShippingStatus:    string(order.ShippingStatus),
		PaymentIntent:     intent,
		Assignment:        firstActiveAssignment(order.Assignments),
		SubtotalCents:     safeCartValue(groupRow, func(g models.CartVendorGroup) int { return g.SubtotalCents }),
		TotalCents:        safeCartValue(groupRow, func(g models.CartVendorGroup) int { return g.TotalCents }),
		Warnings:          safeCartWarnings(groupRow),
		Promo:             safeCartPromo(groupRow),
		Items:             buildLineItemConfirmations(order.Items),
	}
}

func firstActiveAssignment(assignments []models.OrderAssignment) *assignmentSummary {
	for _, assignment := range assignments {
		if !assignment.Active {
			continue
		}
		return &assignmentSummary{
			ID:                      assignment.ID,
			AgentUserID:             assignment.AgentUserID,
			AssignedByUserID:        assignment.AssignedByUserID,
			AssignedAt:              assignment.AssignedAt,
			UnassignedAt:            assignment.UnassignedAt,
			PickupTime:              assignment.PickupTime,
			DeliveryTime:            assignment.DeliveryTime,
			CashPickupTime:          assignment.CashPickupTime,
			PickupSignatureGCSKey:   assignment.PickupSignatureGCSKey,
			DeliverySignatureGCSKey: assignment.DeliverySignatureGCSKey,
		}
	}
	return nil
}

func stringOrEmpty(intent *models.PaymentIntent) string {
	if intent == nil {
		return ""
	}
	return string(intent.Status)
}

func safeCartValue(groupRow *models.CartVendorGroup, fn func(models.CartVendorGroup) int) int {
	if groupRow == nil {
		return 0
	}
	return fn(*groupRow)
}

func safeCartWarnings(groupRow *models.CartVendorGroup) types.VendorGroupWarnings {
	if groupRow == nil {
		return nil
	}
	return groupRow.Warnings
}

func safeCartPromo(groupRow *models.CartVendorGroup) *types.VendorGroupPromo {
	if groupRow == nil {
		return nil
	}
	return groupRow.Promo
}
