package cartdto

import "github.com/google/uuid"

// QuoteCartRequest captures the minimal intent payload for cart quoting.
type QuoteCartRequest struct {
	BuyerStoreID uuid.UUID          `json:"buyer_store_id" validate:"required"`
	Items        []QuoteCartItem    `json:"items" validate:"required,min=1,dive"`
	VendorPromos []QuoteVendorPromo `json:"vendor_promos,omitempty" validate:"omitempty,dive"`
	AdTokens     []string           `json:"ad_tokens,omitempty"`
}

// QuoteCartItem describes a requested product/quantity tuple.
type QuoteCartItem struct {
	ProductID     uuid.UUID `json:"product_id" validate:"required"`
	VendorStoreID uuid.UUID `json:"vendor_store_id" validate:"required"`
	Quantity      int       `json:"quantity" validate:"required,gt=0"`
}

// QuoteVendorPromo pairs a vendor with the promo code the buyer wants to apply.
type QuoteVendorPromo struct {
	VendorStoreID uuid.UUID `json:"vendor_store_id" validate:"required"`
	Code          string    `json:"code" validate:"required"`
}
