package helpers

import (
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/google/uuid"
)

// GroupCartItemsByVendor groups the provided cart items by their vendor store.
func GroupCartItemsByVendor(items []models.CartItem) map[uuid.UUID][]models.CartItem {
	grouped := make(map[uuid.UUID][]models.CartItem, len(items))
	for _, item := range items {
		grouped[item.VendorStoreID] = append(grouped[item.VendorStoreID], item)
	}
	return grouped
}

// VendorCartTotals captures pre-calculated totals for a vendor.
type VendorCartTotals struct {
	VendorStoreID  uuid.UUID
	SubtotalCents  int
	DiscountsCents int
	TotalCents     int
	ItemCount      int
}

// ComputeVendorTotals computes the subtotal, discount, and total for a vendor's cart items.
func ComputeVendorTotals(items []models.CartItem) VendorCartTotals {
	var totals VendorCartTotals
	if len(items) == 0 {
		return totals
	}
	totals.VendorStoreID = items[0].VendorStoreID
	for _, item := range items {
		subtotal := lineSubtotal(item)
		total := lineTotal(item)
		discount := subtotal - total
		if discount < 0 {
			discount = 0
		}
		totals.SubtotalCents += subtotal
		totals.TotalCents += total
		totals.DiscountsCents += discount
		totals.ItemCount++
	}
	return totals
}

// ComputeTotalsByVendor returns pre-computed totals keyed by vendor.
func ComputeTotalsByVendor(items []models.CartItem) map[uuid.UUID]VendorCartTotals {
	results := make(map[uuid.UUID]VendorCartTotals)
	for _, item := range items {
		vendorTotals := results[item.VendorStoreID]
		if vendorTotals.ItemCount == 0 {
			vendorTotals.VendorStoreID = item.VendorStoreID
		}
		subtotal := lineSubtotal(item)
		total := lineTotal(item)
		discount := subtotal - total
		if discount < 0 {
			discount = 0
		}
		vendorTotals.SubtotalCents += subtotal
		vendorTotals.TotalCents += total
		vendorTotals.DiscountsCents += discount
		vendorTotals.ItemCount++
		results[item.VendorStoreID] = vendorTotals
	}
	return results
}

func lineSubtotal(item models.CartItem) int {
	return item.UnitPriceCents * item.Quantity
}

func lineTotal(item models.CartItem) int {
	if item.LineSubtotalCents != 0 {
		return item.LineSubtotalCents
	}
	return lineSubtotal(item)
}
