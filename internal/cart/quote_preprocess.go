package cart

import (
	"context"
	"errors"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type quotePipelineItem struct {
	Request               QuoteCartItem
	Product               *models.Product
	VendorStore           *stores.StoreDTO
	VendorMatch           bool
	ProductAvailable      bool
	NormalizedQty         int
	MOQ                   int
	MaxQty                *int
	Status                enums.CartItemStatus
	Warnings              types.CartItemWarnings
	UnitPriceCents        int
	AppliedVolumeDiscount *types.AppliedVolumeDiscount
	LineSubtotalCents     int
	SelectedTier          *models.ProductVolumeDiscount
}

type quotePipelineResult struct {
	Items         []*quotePipelineItem
	ItemsByVendor map[uuid.UUID][]*quotePipelineItem
}

func (s *service) preprocessQuoteInput(ctx context.Context, buyerState string, input QuoteCartInput, previousPrices map[string]int) (*quotePipelineResult, error) {
	vendorIDs := map[uuid.UUID]struct{}{}
	for _, payload := range input.Items {
		if payload.Quantity <= 0 {
			return nil, pkgerrors.New(pkgerrors.CodeValidation, "item quantity must be positive")
		}
		vendorIDs[payload.VendorStoreID] = struct{}{}
	}

	vendorCache := map[uuid.UUID]*stores.StoreDTO{}
	for vendorID := range vendorIDs {
		if _, err := s.ensureVendor(ctx, vendorID, buyerState, vendorCache); err != nil {
			return nil, err
		}
	}

	result := &quotePipelineResult{
		Items:         make([]*quotePipelineItem, 0, len(input.Items)),
		ItemsByVendor: make(map[uuid.UUID][]*quotePipelineItem, len(vendorIDs)),
	}

	for _, payload := range input.Items {
		vendorStore := vendorCache[payload.VendorStoreID]
		if vendorStore == nil {
			return nil, pkgerrors.New(pkgerrors.CodeValidation, "vendor store missing")
		}

		product, _, err := s.productRepo.GetProductDetail(ctx, payload.ProductID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, pkgerrors.New(pkgerrors.CodeNotFound, "product not found")
			}
			return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load product")
		}

		vendorMatch := product.StoreID == payload.VendorStoreID
		maxQty := productMaxQty(product)
		normalizedQty, warnings := normalizeQuantity(payload.Quantity, product.MOQ, maxQty)
		status := enums.CartItemStatusOK

		if !vendorMatch {
			status = enums.CartItemStatusInvalid
			warnings = appendWarning(warnings, enums.CartItemWarningTypeVendorMismatch, "product does not belong to the requested vendor")
		} else if !product.IsActive || !hasSufficientInventory(product, normalizedQty) {
			status = enums.CartItemStatusNotAvailable
			reason := "product is not active"
			if product.IsActive {
				availableQty := 0
				if product.Inventory != nil {
					availableQty = product.Inventory.AvailableQty
				}
				reason = fmt.Sprintf("product inventory (%d) is below requested quantity (%d)", availableQty, normalizedQty)
			}
			warnings = appendWarning(warnings, enums.CartItemWarningTypeNotAvailable, reason)
		}

		selectedTier := selectVolumeDiscount(normalizedQty, product.VolumeDiscounts)
		unitPrice, appliedDiscount := resolvePricing(product, normalizedQty, selectedTier)
		lineSubtotal := unitPrice * normalizedQty

		key := priceKey(product.ID, payload.VendorStoreID)
		if prevPrice, ok := previousPrices[key]; ok && prevPrice != unitPrice {
			warnings = appendWarning(warnings, enums.CartItemWarningTypePriceChanged, fmt.Sprintf("price changed from %d to %d", prevPrice, unitPrice))
		}

		item := &quotePipelineItem{
			Request:               payload,
			Product:               product,
			VendorStore:           vendorStore,
			VendorMatch:           vendorMatch,
			ProductAvailable:      product.IsActive,
			NormalizedQty:         normalizedQty,
			MOQ:                   product.MOQ,
			MaxQty:                maxQty,
			Status:                status,
			Warnings:              warnings,
			UnitPriceCents:        unitPrice,
			AppliedVolumeDiscount: appliedDiscount,
			LineSubtotalCents:     lineSubtotal,
			SelectedTier:          selectedTier,
		}

		result.Items = append(result.Items, item)
		result.ItemsByVendor[payload.VendorStoreID] = append(result.ItemsByVendor[payload.VendorStoreID], item)
	}

	return result, nil
}

func appendWarning(warnings types.CartItemWarnings, warningType enums.CartItemWarningType, message string) types.CartItemWarnings {
	return append(warnings, types.CartItemWarning{
		Type:    warningType,
		Message: message,
	})
}

func normalizeQuantity(requested, moq int, maxQty *int) (int, types.CartItemWarnings) {
	normalized := requested
	warnings := types.CartItemWarnings{}

	if normalized < moq {
		warnings = appendWarning(warnings, enums.CartItemWarningTypeClampedToMOQ, fmt.Sprintf("quantity raised to MOQ (%d)", moq))
		normalized = moq
	}

	if maxQty != nil && normalized > *maxQty {
		warnings = appendWarning(warnings, enums.CartItemWarningTypeClampedToMax, fmt.Sprintf("quantity reduced to max allowed (%d)", *maxQty))
		normalized = *maxQty
	}

	return normalized, warnings
}

func hasSufficientInventory(product *models.Product, qty int) bool {
	if product == nil {
		return false
	}
	if product.Inventory == nil {
		return false
	}
	return product.Inventory.AvailableQty >= qty
}

// productMaxQty returns the configured product max quantity when available.
func productMaxQty(product *models.Product) *int {
	// TODO: return actual max quantity once the field exists on products or inventory.
	return nil
}

func priceKey(productID, vendorID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", productID, vendorID)
}

func resolvePricing(product *models.Product, qty int, tier *models.ProductVolumeDiscount) (int, *types.AppliedVolumeDiscount) {
	unitPrice := product.PriceCents
	if tier == nil {
		return unitPrice, nil
	}
	diff := product.PriceCents - tier.UnitPriceCents
	if diff < 0 {
		diff = 0
	}
	applied := &types.AppliedVolumeDiscount{
		Label:       fmt.Sprintf("volume tier %d+", tier.MinQty),
		AmountCents: diff * qty,
	}
	return tier.UnitPriceCents, applied
}

func aggregateVendorGroups(pipeline *quotePipelineResult) []models.CartVendorGroup {
	groups := make([]models.CartVendorGroup, 0, len(pipeline.ItemsByVendor))
	for vendorID, items := range pipeline.ItemsByVendor {
		subtotal := 0
		hasOK := false
		for _, item := range items {
			if item.Status == enums.CartItemStatusOK {
				subtotal += item.LineSubtotalCents
				hasOK = true
			}
		}
		status := enums.VendorGroupStatusInvalid
		warnings := types.VendorGroupWarnings{}
		if hasOK {
			status = enums.VendorGroupStatusOK
		} else {
			warnings = append(warnings, types.VendorGroupWarning{
				Type:    enums.VendorGroupWarningTypeVendorInvalid,
				Message: "no valid items for vendor",
			})
		}
		groups = append(groups, models.CartVendorGroup{
			VendorStoreID: vendorID,
			Status:        status,
			Warnings:      warnings,
			SubtotalCents: subtotal,
			TotalCents:    subtotal,
		})
	}
	return groups
}
