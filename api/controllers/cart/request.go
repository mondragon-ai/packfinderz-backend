package cart

import (
	cartdto "github.com/angelmondragon/packfinderz-backend/api/controllers/cart/dto"
	"github.com/angelmondragon/packfinderz-backend/internal/cart"
)

func toQuoteCartInput(payload cartdto.QuoteCartRequest) cart.QuoteCartInput {
	items := make([]cart.QuoteCartItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, cart.QuoteCartItem{
			ProductID:     item.ProductID,
			VendorStoreID: item.VendorStoreID,
			Quantity:      item.Quantity,
		})
	}

	promos := make([]cart.QuoteVendorPromo, 0, len(payload.VendorPromos))
	for _, promo := range payload.VendorPromos {
		promos = append(promos, cart.QuoteVendorPromo{
			VendorStoreID: promo.VendorStoreID,
			Code:          promo.Code,
		})
	}

	return cart.QuoteCartInput{
		Items:        items,
		VendorPromos: promos,
		AdTokens:     payload.AdTokens,
	}
}
