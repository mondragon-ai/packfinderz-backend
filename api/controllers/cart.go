package controllers

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
	"github.com/angelmondragon/packfinderz-backend/api/validators"
	cartsvc "github.com/angelmondragon/packfinderz-backend/internal/cart"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

// CartUpsert handles upsert of the buyer's active cart.
func CartUpsert(svc cartsvc.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "cart service unavailable"))
			return
		}

		buyerStoreID, err := buyerStoreIDFromContext(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		var payload upsertCartRequest
		if err := validators.DecodeJSONBody(r, &payload); err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		input, err := payload.toInput()
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		record, err := svc.UpsertCart(r.Context(), buyerStoreID, input)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, newCartRecordResponse(record))
	}
}

// CartFetch exposes the active cart record for the buyer store.
func CartFetch(svc cartsvc.Service, logg *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			responses.WriteError(r.Context(), logg, w, pkgerrors.New(pkgerrors.CodeInternal, "cart service unavailable"))
			return
		}

		buyerStoreID, err := buyerStoreIDFromContext(r)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		record, err := svc.GetActiveCart(r.Context(), buyerStoreID)
		if err != nil {
			responses.WriteError(r.Context(), logg, w, err)
			return
		}

		responses.WriteSuccess(w, newCartRecordResponse(record))
	}
}

type upsertCartRequest struct {
	ShippingAddress *types.Address    `json:"shipping_address,omitempty"`
	Currency        string            `json:"currency"`
	ValidUntil      *time.Time        `json:"valid_until,omitempty"`
	SubtotalCents   int               `json:"subtotal_cents" validate:"required,min=0"`
	DiscountsCents  int               `json:"discounts_cents" validate:"required,min=0"`
	TotalCents      int               `json:"total_cents" validate:"required,min=0"`
	AdTokens        []string          `json:"ad_tokens"`
	Items           []cartItemPayload `json:"items" validate:"required,dive"`
}

type cartItemPayload struct {
	ProductID                       uuid.UUID                    `json:"product_id" validate:"required"`
	VendorStoreID                   uuid.UUID                    `json:"vendor_store_id" validate:"required"`
	Qty                             int                          `json:"qty" validate:"required,min=1"`
	UnitPriceCents                  int                          `json:"unit_price_cents" validate:"required"`
	AppliedVolumeTierMinQty         *int                         `json:"applied_volume_tier_min_qty"`
	AppliedVolumeTierUnitPriceCents *int                         `json:"applied_volume_tier_unit_price_cents"`
	DiscountedPrice                 *int                         `json:"discounted_price"`
	SubTotalPrice                   *int                         `json:"sub_total_price" validate:"required"`
	AppliedVolumeDiscount           *types.AppliedVolumeDiscount `json:"applied_volume_discount"`
}

func (r upsertCartRequest) toInput() (cartsvc.UpsertCartInput, error) {
	items := make([]cartsvc.CartItemInput, len(r.Items))
	for i, payload := range r.Items {
		items[i] = cartsvc.CartItemInput{
			ProductID:                       payload.ProductID,
			VendorStoreID:                   payload.VendorStoreID,
			Qty:                             payload.Qty,
			UnitPriceCents:                  payload.UnitPriceCents,
			AppliedVolumeTierMinQty:         payload.AppliedVolumeTierMinQty,
			AppliedVolumeTierUnitPriceCents: payload.AppliedVolumeTierUnitPriceCents,
			DiscountedPrice:                 payload.DiscountedPrice,
			SubTotalPrice:                   payload.SubTotalPrice,
			AppliedVolumeDiscount:           payload.AppliedVolumeDiscount,
		}
	}

	return cartsvc.UpsertCartInput{
		ShippingAddress: r.ShippingAddress,
		Currency:        r.Currency,
		ValidUntil:      r.ValidUntil,
		DiscountsCents:  r.DiscountsCents,
		SubtotalCents:   r.SubtotalCents,
		TotalCents:      r.TotalCents,
		AdTokens:        r.AdTokens,
		Items:           items,
	}, nil
}

type cartRecordResponse struct {
	ID              uuid.UUID                 `json:"id"`
	BuyerStoreID    uuid.UUID                 `json:"buyer_store_id"`
	CheckoutGroupID *uuid.UUID                `json:"checkout_group_id,omitempty"`
	Status          string                    `json:"status"`
	ShippingAddress *types.Address            `json:"shipping_address,omitempty"`
	Currency        string                    `json:"currency"`
	ValidUntil      time.Time                 `json:"valid_until"`
	SubtotalCents   int                       `json:"subtotal_cents"`
	DiscountsCents  int                       `json:"discounts_cents"`
	TotalCents      int                       `json:"total_cents"`
	AdTokens        []string                  `json:"ad_tokens,omitempty"`
	VendorGroups    []cartVendorGroupResponse `json:"vendor_groups,omitempty"`
	Items           []cartItemResponse        `json:"items"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

type cartVendorGroupResponse struct {
	VendorStoreID uuid.UUID                 `json:"vendor_store_id"`
	Status        string                    `json:"status"`
	Warnings      types.VendorGroupWarnings `json:"warnings,omitempty"`
	SubtotalCents int                       `json:"subtotal_cents"`
	Promo         *types.VendorGroupPromo   `json:"promo,omitempty"`
	TotalCents    int                       `json:"total_cents"`
}

type cartItemResponse struct {
	ID                    uuid.UUID                    `json:"id"`
	ProductID             uuid.UUID                    `json:"product_id"`
	VendorStoreID         uuid.UUID                    `json:"vendor_store_id"`
	Quantity              int                          `json:"quantity"`
	MOQ                   int                          `json:"moq"`
	MaxQty                *int                         `json:"max_qty,omitempty"`
	UnitPriceCents        int                          `json:"unit_price_cents"`
	AppliedVolumeDiscount *types.AppliedVolumeDiscount `json:"applied_volume_discount,omitempty"`
	LineSubtotalCents     int                          `json:"line_subtotal_cents"`
	Status                string                       `json:"status"`
	Warnings              types.CartItemWarnings       `json:"warnings,omitempty"`
	CreatedAt             time.Time                    `json:"created_at"`
	UpdatedAt             time.Time                    `json:"updated_at"`
}

func newCartRecordResponse(record *models.CartRecord) cartRecordResponse {
	items := make([]cartItemResponse, 0, len(record.Items))
	for _, item := range record.Items {
		items = append(items, cartItemResponse{
			ID:                    item.ID,
			ProductID:             item.ProductID,
			VendorStoreID:         item.VendorStoreID,
			Quantity:              item.Quantity,
			MOQ:                   item.MOQ,
			MaxQty:                item.MaxQty,
			UnitPriceCents:        item.UnitPriceCents,
			AppliedVolumeDiscount: item.AppliedVolumeDiscount,
			LineSubtotalCents:     item.LineSubtotalCents,
			Status:                string(item.Status),
			Warnings:              item.Warnings,
			CreatedAt:             item.CreatedAt,
			UpdatedAt:             item.UpdatedAt,
		})
	}

	vendorGroups := make([]cartVendorGroupResponse, 0, len(record.VendorGroups))
	for _, group := range record.VendorGroups {
		vendorGroups = append(vendorGroups, cartVendorGroupResponse{
			VendorStoreID: group.VendorStoreID,
			Status:        string(group.Status),
			Warnings:      group.Warnings,
			SubtotalCents: group.SubtotalCents,
			Promo:         group.Promo,
			TotalCents:    group.TotalCents,
		})
	}

	return cartRecordResponse{
		ID:              record.ID,
		BuyerStoreID:    record.BuyerStoreID,
		CheckoutGroupID: record.CheckoutGroupID,
		Status:          string(record.Status),
		ShippingAddress: record.ShippingAddress,
		Currency:        record.Currency,
		ValidUntil:      record.ValidUntil,
		SubtotalCents:   record.SubtotalCents,
		DiscountsCents:  record.DiscountsCents,
		TotalCents:      record.TotalCents,
		AdTokens:        []string(record.AdTokens),
		VendorGroups:    vendorGroups,
		Items:           items,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
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
