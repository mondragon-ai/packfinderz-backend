package cart

import (
	"time"

	"github.com/google/uuid"

	cartsvc "github.com/angelmondragon/packfinderz-backend/internal/cart"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

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
