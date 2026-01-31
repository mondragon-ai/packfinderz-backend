package cartdto

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

// CartQuote represents the authoritative cart snapshot exposed through the API.
type CartQuote struct {
	ID              uuid.UUID              `json:"id"`
	BuyerStoreID    uuid.UUID              `json:"buyer_store_id"`
	CheckoutGroupID *uuid.UUID             `json:"checkout_group_id,omitempty"`
	Status          enums.CartStatus       `json:"status"`
	ShippingAddress *types.Address         `json:"shipping_address,omitempty"`
	Currency        string                 `json:"currency"`
	ValidUntil      time.Time              `json:"valid_until"`
	SubtotalCents   int                    `json:"subtotal_cents"`
	DiscountsCents  int                    `json:"discounts_cents"`
	TotalCents      int                    `json:"total_cents"`
	AdTokens        []string               `json:"ad_tokens,omitempty"`
	VendorGroups    []CartQuoteVendorGroup `json:"vendor_groups,omitempty"`
	Items           []CartQuoteItem        `json:"items"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// CartQuoteVendorGroup captures vendor-level meta inside the cart quote.
type CartQuoteVendorGroup struct {
	VendorStoreID uuid.UUID                 `json:"vendor_store_id"`
	Status        enums.VendorGroupStatus   `json:"status"`
	Warnings      types.VendorGroupWarnings `json:"warnings,omitempty"`
	SubtotalCents int                       `json:"subtotal_cents"`
	Promo         *types.VendorGroupPromo   `json:"promo,omitempty"`
	TotalCents    int                       `json:"total_cents"`
}

// CartQuoteItem describes each line item in the authoritative quote.
type CartQuoteItem struct {
	ID                    uuid.UUID                    `json:"id"`
	ProductID             uuid.UUID                    `json:"product_id"`
	VendorStoreID         uuid.UUID                    `json:"vendor_store_id"`
	Quantity              int                          `json:"quantity"`
	MOQ                   int                          `json:"moq"`
	MaxQty                *int                         `json:"max_qty,omitempty"`
	UnitPriceCents        int                          `json:"unit_price_cents"`
	AppliedVolumeDiscount *types.AppliedVolumeDiscount `json:"applied_volume_discount,omitempty"`
	LineSubtotalCents     int                          `json:"line_subtotal_cents"`
	Status                enums.CartItemStatus         `json:"status"`
	Warnings              types.CartItemWarnings       `json:"warnings,omitempty"`
	CreatedAt             time.Time                    `json:"created_at"`
	UpdatedAt             time.Time                    `json:"updated_at"`
}
