package cartdto

import "github.com/google/uuid"

// QuoteCartRequest captures the minimal intent payload for cart quoting.
type QuoteCartRequest struct {
	BuyerStoreID uuid.UUID          `json:"buyer_store_id"`
	Items        []QuoteCartItem    `json:"items"`
	VendorPromos []QuoteVendorPromo `json:"vendor_promos,omitempty"`
	AdTokens     []string           `json:"ad_tokens,omitempty"`
}

// QuoteCartItem describes a requested product/quantity tuple.
type QuoteCartItem struct {
	ProductID     uuid.UUID `json:"product_id"`
	VendorStoreID uuid.UUID `json:"vendor_store_id"`
	Quantity      int       `json:"quantity"`
}

// QuoteVendorPromo pairs a vendor with the promo code the buyer wants to apply.
type QuoteVendorPromo struct {
	VendorStoreID uuid.UUID `json:"vendor_store_id"`
	Code          string    `json:"code"`
}
