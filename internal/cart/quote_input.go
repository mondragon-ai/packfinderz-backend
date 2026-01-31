package cart

import "github.com/google/uuid"

// QuoteCartInput represents the server-driven quote intent derived from cartdto.QuoteCartRequest.
type QuoteCartInput struct {
	Items        []QuoteCartItem
	VendorPromos []QuoteVendorPromo
	AdTokens     []string
}

// QuoteCartItem captures each intent line from the client.
type QuoteCartItem struct {
	ProductID     uuid.UUID
	VendorStoreID uuid.UUID
	Quantity      int
}

// QuoteVendorPromo pairs a vendor with a promo code supplied in the quote request.
type QuoteVendorPromo struct {
	VendorStoreID uuid.UUID
	Code          string
}
