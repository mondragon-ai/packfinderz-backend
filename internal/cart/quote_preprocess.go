package cart

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type quotePipelineItem struct {
	Request                 QuoteCartItem
	Product                 *models.Product
	VendorStore             *stores.StoreDTO
	VendorMatch             bool
	ProductAvailable        bool
	Title                   string
	Thumbnail               *string
	NormalizedQty           int
	MOQ                     int
	MaxQty                  *int
	Status                  enums.CartItemStatus
	Warnings                types.CartItemWarnings
	UnitPriceCents          int
	EffectiveUnitPriceCents int
	LineDiscountsCents      int
	LineTotalCents          int
	AppliedVolumeDiscount   *types.AppliedVolumeDiscount
	LineSubtotalCents       int
	SelectedTier            *models.ProductVolumeDiscount
}

type quotePipelineResult struct {
	Items          []*quotePipelineItem
	ItemsByVendor  map[uuid.UUID][]*quotePipelineItem
	VendorWarnings map[uuid.UUID]types.VendorGroupWarnings
	VendorPromos   map[uuid.UUID]*types.VendorGroupPromo
}

const invalidPromoWarningMessage = "Promo code is not valid for this vendor"

func (s *service) preprocessQuoteInput(ctx context.Context, buyerState string, input QuoteCartInput, previousPrices map[string]int) (*quotePipelineResult, error) {
	vendorIDs := map[uuid.UUID]struct{}{}
	for _, payload := range input.Items {
		if payload.Quantity <= 0 {
			return nil, pkgerrors.New(pkgerrors.CodeValidation, "item quantity must be positive")
		}
		vendorIDs[payload.VendorStoreID] = struct{}{}
	}

	promoRequests := map[uuid.UUID]QuoteVendorPromo{}
	for _, promo := range input.VendorPromos {
		if promo.VendorStoreID == uuid.Nil {
			continue
		}
		if _, ok := vendorIDs[promo.VendorStoreID]; !ok {
			continue
		}
		promoRequests[promo.VendorStoreID] = promo
	}

	vendorCache := map[uuid.UUID]*stores.StoreDTO{}
	for vendorID := range vendorIDs {
		if _, err := s.ensureVendor(ctx, vendorID, buyerState, vendorCache); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	vendorWarnings := make(map[uuid.UUID]types.VendorGroupWarnings, len(vendorIDs))
	vendorPromos := make(map[uuid.UUID]*types.VendorGroupPromo, len(vendorIDs))
	for vendorID, promo := range promoRequests {
		promoRecord, err := s.promo.GetVendorPromo(ctx, vendorID, promo.Code)
		if err != nil {
			return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load vendor promo")
		}
		if promoRecord == nil || promoRecord.VendorStoreID != vendorID || !promoRecord.IsValid(now) {
			vendorWarnings[vendorID] = append(vendorWarnings[vendorID], types.VendorGroupWarning{
				Type:    enums.VendorGroupWarningTypeInvalidPromo,
				Message: invalidPromoWarningMessage,
			})
			continue
		}

		amount := promoRecord.AmountCents
		if amount < 0 {
			amount = 0
		}

		vendorPromos[vendorID] = &types.VendorGroupPromo{
			Code:        promoRecord.Code,
			AmountCents: amount,
		}
	}

	result := &quotePipelineResult{
		Items:          make([]*quotePipelineItem, 0, len(input.Items)),
		ItemsByVendor:  make(map[uuid.UUID][]*quotePipelineItem, len(vendorIDs)),
		VendorWarnings: vendorWarnings,
		VendorPromos:   vendorPromos,
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

		baseUnitPriceCents, _, effectiveUnitPriceCents, applied :=
			resolvePricing(product, normalizedQty, selectedTier)

		lineSubtotalCents := baseUnitPriceCents * normalizedQty
		if lineSubtotalCents < 0 {
			lineSubtotalCents = 0
		}

		lineTotalCents := effectiveUnitPriceCents * normalizedQty
		if lineTotalCents < 0 {
			lineTotalCents = 0
		}

		lineDiscountsCents := lineSubtotalCents - lineTotalCents
		if lineDiscountsCents < 0 {
			lineDiscountsCents = 0
		}
		if applied != nil {
			applied.AmountCents = lineDiscountsCents
		}

		key := priceKey(product.ID, payload.VendorStoreID)
		if prevPrice, ok := previousPrices[key]; ok && prevPrice != baseUnitPriceCents {
			warnings = appendWarning(
				warnings,
				enums.CartItemWarningTypePriceChanged,
				fmt.Sprintf("price changed from %d to %d", prevPrice, baseUnitPriceCents),
			)
		}

		item := &quotePipelineItem{
			Request:          payload,
			Product:          product,
			VendorStore:      vendorStore,
			VendorMatch:      vendorMatch,
			ProductAvailable: product.IsActive,

			Title:     product.Title,
			Thumbnail: firstMediaURL(product),

			NormalizedQty: normalizedQty,
			MOQ:           product.MOQ,
			MaxQty:        maxQty,
			Status:        status,
			Warnings:      warnings,

			UnitPriceCents: baseUnitPriceCents,

			EffectiveUnitPriceCents: effectiveUnitPriceCents,
			LineDiscountsCents:      lineDiscountsCents,
			LineTotalCents:          lineTotalCents,

			AppliedVolumeDiscount: applied,
			LineSubtotalCents:     lineSubtotalCents,
			SelectedTier:          selectedTier,
		}

		result.Items = append(result.Items, item)
		result.ItemsByVendor[payload.VendorStoreID] = append(result.ItemsByVendor[payload.VendorStoreID], item)
	}

	return result, nil
}

func firstMediaURL(product *models.Product) *string {
	if product == nil || len(product.Media) == 0 || product.Media[0].URL == nil {
		return nil
	}

	u := strings.TrimSpace(*product.Media[0].URL)
	if u == "" {
		return nil
	}

	product.Media[0].URL = &u
	return product.Media[0].URL
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

func resolvePricing(
	product *models.Product,
	qty int,
	tier *models.ProductVolumeDiscount,
) (baseUnitPriceCents int, lineDiscountsCents int, effectiveUnitPriceCents int, applied *types.AppliedVolumeDiscount) {
	if product == nil {
		return 0, 0, 0, nil
	}
	if qty < 0 {
		qty = 0
	}

	base := product.PriceCents
	if base < 0 {
		base = 0
	}

	effective := base
	lineDiscounts := 0
	var appliedDiscount *types.AppliedVolumeDiscount

	if tier != nil && tier.DiscountPercent > 0 {
		discountPerUnit := int(math.Round(float64(base) * float64(tier.DiscountPercent) / 100.0))
		if discountPerUnit < 0 {
			discountPerUnit = 0
		}
		if discountPerUnit > base {
			discountPerUnit = base
		}

		effective = base - discountPerUnit
		if effective < 0 {
			effective = 0
		}

		lineDiscounts = discountPerUnit * qty
		if lineDiscounts < 0 {
			lineDiscounts = 0
		}

		appliedDiscount = &types.AppliedVolumeDiscount{
			Label:       fmt.Sprintf("volume tier %d+", tier.MinQty),
			AmountCents: lineDiscounts,
		}
	}

	return base, lineDiscounts, effective, appliedDiscount
}

func aggregateVendorGroups(pipeline *quotePipelineResult) []models.CartVendorGroup {
	groups := make([]models.CartVendorGroup, 0, len(pipeline.ItemsByVendor))

	for vendorID, items := range pipeline.ItemsByVendor {
		subtotal := 0
		lineDiscounts := 0
		hasOK := false

		for _, item := range items {
			if item.Status != enums.CartItemStatusOK {
				continue
			}
			hasOK = true
			subtotal += item.LineSubtotalCents
			lineDiscounts += item.LineDiscountsCents
		}

		if subtotal < 0 {
			subtotal = 0
		}
		if lineDiscounts < 0 {
			lineDiscounts = 0
		}
		if lineDiscounts > subtotal {
			lineDiscounts = subtotal
		}

		status := enums.VendorGroupStatusInvalid
		warnings := append(types.VendorGroupWarnings{}, pipeline.VendorWarnings[vendorID]...)

		if hasOK {
			status = enums.VendorGroupStatusOK
		} else {
			warnings = append(warnings, types.VendorGroupWarning{
				Type:    enums.VendorGroupWarningTypeVendorInvalid,
				Message: "no valid items for vendor",
			})
		}

		promo := pipeline.VendorPromos[vendorID]

		promoDiscount := 0
		if promo != nil && promo.AmountCents > 0 {
			remaining := subtotal - lineDiscounts
			if remaining < 0 {
				remaining = 0
			}
			promoDiscount = promo.AmountCents
			if promoDiscount < 0 {
				promoDiscount = 0
			}
			if promoDiscount > remaining {
				promoDiscount = remaining
			}
		}

		discounts := lineDiscounts + promoDiscount
		if discounts < 0 {
			discounts = 0
		}
		if discounts > subtotal {
			discounts = subtotal
		}

		total := subtotal - discounts
		if total < 0 {
			total = 0
		}

		groups = append(groups, models.CartVendorGroup{
			VendorStoreID:      vendorID,
			Status:             status,
			Warnings:           warnings,
			SubtotalCents:      subtotal,
			Promo:              promo,
			LineDiscountsCents: lineDiscounts,
			PromoDiscountCents: promoDiscount,
			DiscountsCents:     discounts,
			TotalCents:         total,
		})
	}

	return groups
}
