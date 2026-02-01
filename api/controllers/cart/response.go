package cart

import (
	cartdto "github.com/angelmondragon/packfinderz-backend/api/controllers/cart/dto"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
)

func newCartQuote(record *models.CartRecord) cartdto.CartQuote {
	items := make([]cartdto.CartQuoteItem, 0, len(record.Items))
	for _, item := range record.Items {
		items = append(items, cartdto.CartQuoteItem{
			ID:                    item.ID,
			ProductID:             item.ProductID,
			VendorStoreID:         item.VendorStoreID,
			Quantity:              item.Quantity,
			MOQ:                   item.MOQ,
			MaxQty:                item.MaxQty,
			UnitPriceCents:        item.UnitPriceCents,
			AppliedVolumeDiscount: item.AppliedVolumeDiscount,
			LineSubtotalCents:     item.LineSubtotalCents,
			Status:                item.Status,
			Warnings:              item.Warnings,
			CreatedAt:             item.CreatedAt,
			UpdatedAt:             item.UpdatedAt,
		})
	}

	vendorGroups := make([]cartdto.CartQuoteVendorGroup, 0, len(record.VendorGroups))
	for _, group := range record.VendorGroups {
		vendorGroups = append(vendorGroups, cartdto.CartQuoteVendorGroup{
			VendorStoreID: group.VendorStoreID,
			Status:        group.Status,
			Warnings:      group.Warnings,
			SubtotalCents: group.SubtotalCents,
			Promo:         group.Promo,
			TotalCents:    group.TotalCents,
		})
	}

	return cartdto.CartQuote{
		ID:              record.ID,
		BuyerStoreID:    record.BuyerStoreID,
		CheckoutGroupID: record.CheckoutGroupID,
		Status:          record.Status,
		ShippingAddress: record.ShippingAddress,
		Currency:        string(record.Currency),
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
