package cart

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	checkouthelpers "github.com/angelmondragon/packfinderz-backend/internal/checkout/helpers"
	product "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/checkout"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type txRunner interface {
	WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type storeLoader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error)
}

type productLoader interface {
	GetProductDetail(ctx context.Context, id uuid.UUID) (*models.Product, *product.VendorSummary, error)
}

// Service exposes cart persistence operations.
type Service interface {
	UpsertCart(ctx context.Context, buyerStoreID uuid.UUID, input UpsertCartInput) (*models.CartRecord, error)
	QuoteCart(ctx context.Context, buyerStoreID uuid.UUID, input QuoteCartInput) (*models.CartRecord, error)
	GetActiveCart(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error)
}

type service struct {
	repo        CartRepository
	tx          txRunner
	store       storeLoader
	productRepo productLoader
}

// NewService builds a cart service backed by the provided stack.
func NewService(repo CartRepository, tx txRunner, store storeLoader, productRepo productLoader) (Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("cart repository required")
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction runner required")
	}
	if store == nil {
		return nil, fmt.Errorf("store loader required")
	}
	if productRepo == nil {
		return nil, fmt.Errorf("product loader required")
	}
	return &service{
		repo:        repo,
		tx:          tx,
		store:       store,
		productRepo: productRepo,
	}, nil
}

// UpsertCartInput captures the payload required to create or refresh a cart record.
type UpsertCartInput struct {
	ShippingAddress         *types.Address
	Currency                enums.Currency
	ValidUntil              *time.Time
	DiscountsCents          int
	SubtotalCents           int
	TotalCents              int
	AdTokens                []string
	Items                   []CartItemInput
	SkipTotalsValidation    bool
	SkipInventoryValidation bool
	SkipVendorValidation    bool
	VendorGroups            []models.CartVendorGroup
}

// CartItemInput mirrors the data stored for each cart item.
type CartItemInput struct {
	ProductID                       uuid.UUID
	VendorStoreID                   uuid.UUID
	Qty                             int
	MOQ                             int
	MaxQty                          *int
	Status                          enums.CartItemStatus
	Warnings                        types.CartItemWarnings
	ProductSKU                      string
	Unit                            enums.ProductUnit
	UnitPriceCents                  int
	CompareAtUnitPriceCents         *int
	AppliedVolumeTierMinQty         *int
	AppliedVolumeTierUnitPriceCents *int
	DiscountedPrice                 *int
	SubTotalPrice                   *int
	AppliedVolumeDiscount           *types.AppliedVolumeDiscount
	FeaturedImage                   *string
	THCPercent                      *float64
	CBDPercent                      *float64
}

// UpsertCart validates the provided snapshot and persists the cart atomically.
func (s *service) UpsertCart(ctx context.Context, buyerStoreID uuid.UUID, input UpsertCartInput) (*models.CartRecord, error) {
	if buyerStoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "buyer store id is required")
	}
	if len(input.Items) == 0 {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "cart must contain at least one item")
	}
	if input.SubtotalCents < 0 || input.TotalCents < 0 || input.DiscountsCents < 0 {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "cart totals must be non-negative")
	}

	_, buyerState, err := s.validateBuyerStore(ctx, buyerStoreID)
	if err != nil {
		return nil, err
	}

	vendorCache := map[uuid.UUID]*stores.StoreDTO{}
	vendorIDs := map[uuid.UUID]struct{}{}
	var moqInputs []checkout.MOQValidationInput
	var subtotalSum int64
	items := make([]models.CartItem, 0, len(input.Items))

	for _, payload := range input.Items {
		vendorIDs[payload.VendorStoreID] = struct{}{}

		if !input.SkipVendorValidation {
			vendor, err := s.ensureVendor(ctx, payload.VendorStoreID, buyerState, vendorCache)
			if err != nil {
				return nil, err
			}
			_ = vendor // ensure we touched vendor to avoid unused
		}

		product, _, err := s.productRepo.GetProductDetail(ctx, payload.ProductID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, pkgerrors.New(pkgerrors.CodeNotFound, "product not found")
			}
			return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load product")
		}

		if product.StoreID != payload.VendorStoreID {
			return nil, pkgerrors.New(pkgerrors.CodeValidation, "product vendor mismatch")
		}
		if !product.IsActive {
			return nil, pkgerrors.New(pkgerrors.CodeValidation, "product is not available")
		}
		// if payload.ProductSKU != product.SKU {
		// 	return nil, pkgerrors.New(pkgerrors.CodeValidation, "product sku mismatch")
		// }
		// if payload.UnitPriceCents != product.PriceCents {
		// 	return nil, pkgerrors.New(pkgerrors.CodeValidation, "unit price mismatch")
		// }

		availableQty := 0
		if product.Inventory != nil {
			availableQty = product.Inventory.AvailableQty
		}
		if payload.Qty > 0 && availableQty < payload.Qty && !input.SkipInventoryValidation {
			return nil, pkgerrors.New(pkgerrors.CodeConflict, "insufficient inventory for product")
		}

		tier := selectVolumeDiscount(payload.Qty, product.VolumeDiscounts)
		if err := s.validateVolumeTier(payload, tier); err != nil {
			return nil, err
		}

		linePrice := payload.UnitPriceCents
		if payload.DiscountedPrice != nil {
			linePrice = *payload.DiscountedPrice
		}
		if linePrice < 0 {
			return nil, pkgerrors.New(pkgerrors.CodeValidation, "line price cannot be negative")
		}
		lineTotal := linePrice * payload.Qty
		if payload.SubTotalPrice == nil {
			return nil, pkgerrors.New(pkgerrors.CodeValidation, "sub_total_price is required")
		}
		if *payload.SubTotalPrice != lineTotal {
			return nil, pkgerrors.New(pkgerrors.CodeValidation, "line subtotal mismatch")
		}

		subtotalSum += int64(lineTotal)

		moqInputs = append(moqInputs, checkout.MOQValidationInput{
			ProductID:   product.ID,
			ProductName: product.Title,
			MOQ:         product.MOQ,
			Quantity:    payload.Qty,
		})

		items = append(items, buildCartItem(payload, product, lineTotal))
	}

	if err := checkout.ValidateMOQ(moqInputs); err != nil {
		return nil, err
	}

	if !input.SkipTotalsValidation {
		if err := verifyTotals(input, subtotalSum); err != nil {
			return nil, err
		}
	}

	var saved *models.CartRecord
	if err := s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		txRepo := s.repo.WithTx(tx)
		record, err := txRepo.FindActiveByBuyerStore(ctx, buyerStoreID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var targetID uuid.UUID
		validUntil := time.Now().Add(15 * time.Minute)
		if input.ValidUntil != nil {
			validUntil = *input.ValidUntil
		}

		if record != nil {
			record.ShippingAddress = input.ShippingAddress
			if input.Currency.IsValid() {
				record.Currency = input.Currency
			}
			record.ValidUntil = validUntil
			record.SubtotalCents = input.SubtotalCents
			record.DiscountsCents = input.DiscountsCents
			record.TotalCents = input.TotalCents
			record.AdTokens = pq.StringArray(input.AdTokens)
			if _, err := txRepo.Update(ctx, record); err != nil {
				return err
			}
			targetID = record.ID
		} else {
			currency := input.Currency
			if !currency.IsValid() {
				currency = enums.CurrencyUSD
			}
			record = &models.CartRecord{
				BuyerStoreID:    buyerStoreID,
				ShippingAddress: input.ShippingAddress,
				Currency:        currency,
				ValidUntil:      validUntil,
				SubtotalCents:   input.SubtotalCents,
				DiscountsCents:  input.DiscountsCents,
				TotalCents:      input.TotalCents,
				AdTokens:        pq.StringArray(input.AdTokens),
			}
			created, err := txRepo.Create(ctx, record)
			if err != nil {
				return err
			}
			targetID = created.ID
		}

		for i := range items {
			items[i].CartID = targetID
		}

		if err := txRepo.ReplaceItems(ctx, targetID, items); err != nil {
			return err
		}
		if err := txRepo.ReplaceVendorGroups(ctx, targetID, input.VendorGroups); err != nil {
			return err
		}

		saved, err = txRepo.FindByIDAndBuyerStore(ctx, targetID, buyerStoreID)
		return err
	}); err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "persist cart")
	}

	return saved, nil
}

// GetActiveCart returns the active cart for the buyer, or not-found.
func (s *service) GetActiveCart(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	if buyerStoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "buyer store id is required")
	}

	if _, _, err := s.validateBuyerStore(ctx, buyerStoreID); err != nil {
		return nil, err
	}

	record, err := s.repo.FindActiveByBuyerStore(ctx, buyerStoreID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.New(pkgerrors.CodeNotFound, "active cart not found")
		}
		return nil, err
	}
	return record, nil
}

// QuoteCart builds a runnable quote intent from the minimal request shape and persists it via UpsertCart.
func (s *service) QuoteCart(ctx context.Context, buyerStoreID uuid.UUID, input QuoteCartInput) (*models.CartRecord, error) {
	if buyerStoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "buyer store id is required")
	}
	if len(input.Items) == 0 {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "cart must contain at least one item")
	}

	store, buyerState, err := s.validateBuyerStore(ctx, buyerStoreID)
	if err != nil {
		return nil, err
	}

	existingPrices, err := s.loadExistingItemPrices(ctx, buyerStoreID)
	if err != nil {
		return nil, err
	}

	pipeline, err := s.preprocessQuoteInput(ctx, buyerState, input, existingPrices)
	if err != nil {
		return nil, err
	}

	items := make([]CartItemInput, 0, len(pipeline.Items))

	for _, pipelineItem := range pipeline.Items {
		product := pipelineItem.Product
		qty := pipelineItem.NormalizedQty
		tier := pipelineItem.SelectedTier
		lineSubtotal := pipelineItem.LineSubtotalCents
		var discountedPrice *int
		var tierMinQty *int
		var tierUnitPrice *int
		if tier != nil {
			minQty := tier.MinQty
			tierMinQty = &minQty
			unitPrice := tier.UnitPriceCents
			tierUnitPrice = &unitPrice
			discountedPrice = &unitPrice
		}

		items = append(items, CartItemInput{
			ProductID:                       product.ID,
			VendorStoreID:                   product.StoreID,
			Qty:                             qty,
			MOQ:                             pipelineItem.MOQ,
			MaxQty:                          pipelineItem.MaxQty,
			Status:                          pipelineItem.Status,
			Warnings:                        pipelineItem.Warnings,
			ProductSKU:                      product.SKU,
			Unit:                            product.Unit,
			UnitPriceCents:                  pipelineItem.UnitPriceCents,
			CompareAtUnitPriceCents:         product.CompareAtPriceCents,
			AppliedVolumeTierMinQty:         tierMinQty,
			AppliedVolumeTierUnitPriceCents: tierUnitPrice,
			DiscountedPrice:                 discountedPrice,
			SubTotalPrice:                   &lineSubtotal,
			AppliedVolumeDiscount:           pipelineItem.AppliedVolumeDiscount,
			FeaturedImage:                   nil,
			THCPercent:                      product.THCPercent,
			CBDPercent:                      product.CBDPercent,
		})

		if err := s.validateVolumeTier(items[len(items)-1], tier); err != nil {
			return nil, err
		}
	}

	vendorGroups := aggregateVendorGroups(pipeline)
	subtotalCents := 0
	for _, group := range vendorGroups {
		subtotalCents += group.SubtotalCents
	}
	discountsCents := 0
	totalCents := subtotalCents - discountsCents
	if totalCents < 0 {
		totalCents = 0
	}
	shippingAddress := store.Address
	validUntil := time.Now().Add(15 * time.Minute)
	currency := enums.CurrencyUSD

	upsertInput := UpsertCartInput{
		ShippingAddress:         &shippingAddress,
		Currency:                currency,
		ValidUntil:              &validUntil,
		DiscountsCents:          discountsCents,
		SubtotalCents:           subtotalCents,
		TotalCents:              totalCents,
		AdTokens:                input.AdTokens,
		Items:                   items,
		SkipTotalsValidation:    true,
		SkipInventoryValidation: true,
		SkipVendorValidation:    true,
		VendorGroups:            vendorGroups,
	}

	return s.UpsertCart(ctx, buyerStoreID, upsertInput)
}

func (s *service) validateBuyerStore(ctx context.Context, buyerStoreID uuid.UUID) (*stores.StoreDTO, string, error) {
	store, err := s.store.GetByID(ctx, buyerStoreID)
	if err != nil {
		return nil, "", err
	}
	if store.Type != enums.StoreTypeBuyer {
		return nil, "", pkgerrors.New(pkgerrors.CodeForbidden, "active store must be a buyer")
	}
	if store.KYCStatus != enums.KYCStatusVerified {
		return nil, "", pkgerrors.New(pkgerrors.CodeForbidden, "buyer store must be verified")
	}
	state := normalizeState(store.Address.State)
	if state == "" {
		return nil, "", pkgerrors.New(pkgerrors.CodeValidation, "buyer store state is required")
	}
	return store, state, nil
}

func (s *service) ensureVendor(ctx context.Context, vendorID uuid.UUID, buyerState string, cache map[uuid.UUID]*stores.StoreDTO) (*stores.StoreDTO, error) {
	if cached, ok := cache[vendorID]; ok {
		return cached, nil
	}

	vendor, err := s.store.GetByID(ctx, vendorID)
	if err != nil {
		return nil, err
	}

	if err := checkouthelpers.ValidateVendorStore(vendor, buyerState); err != nil {
		return nil, err
	}

	cache[vendorID] = vendor
	return vendor, nil
}

func (s *service) loadExistingItemPrices(ctx context.Context, buyerStoreID uuid.UUID) (map[string]int, error) {
	record, err := s.repo.FindActiveByBuyerStore(ctx, buyerStoreID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return map[string]int{}, nil
		}
		return nil, err
	}
	prices := make(map[string]int, len(record.Items))
	for _, item := range record.Items {
		prices[priceKey(item.ProductID, item.VendorStoreID)] = item.UnitPriceCents
	}
	return prices, nil
}

func (s *service) validateVolumeTier(input CartItemInput, tier *models.ProductVolumeDiscount) error {
	if tier == nil {
		if input.AppliedVolumeTierMinQty != nil || input.AppliedVolumeTierUnitPriceCents != nil {
			return pkgerrors.New(pkgerrors.CodeValidation, "unexpected volume tier data")
		}
		return nil
	}
	if input.AppliedVolumeTierMinQty == nil || input.AppliedVolumeTierUnitPriceCents == nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "volume tier data incomplete")
	}
	if *input.AppliedVolumeTierMinQty != tier.MinQty || *input.AppliedVolumeTierUnitPriceCents != tier.UnitPriceCents {
		return pkgerrors.New(pkgerrors.CodeValidation, "volume tier mismatch")
	}
	if input.DiscountedPrice == nil || *input.DiscountedPrice != tier.UnitPriceCents {
		return pkgerrors.New(pkgerrors.CodeValidation, "discounted price must match volume tier")
	}
	return nil
}

func verifyTotals(input UpsertCartInput, subtotal int64) error {
	if int64(input.SubtotalCents) != subtotal {
		return pkgerrors.New(pkgerrors.CodeValidation, "cart subtotal mismatch")
	}
	if input.DiscountsCents > input.SubtotalCents {
		return pkgerrors.New(pkgerrors.CodeValidation, "discounts exceed subtotal")
	}
	if input.SubtotalCents-input.DiscountsCents != input.TotalCents {
		return pkgerrors.New(pkgerrors.CodeValidation, "cart total mismatch")
	}
	return nil
}

func buildCartItem(input CartItemInput, product *models.Product, lineTotal int) models.CartItem {
	return models.CartItem{
		ProductID:             product.ID,
		VendorStoreID:         product.StoreID,
		Quantity:              input.Qty,
		MOQ:                   input.MOQ,
		MaxQty:                input.MaxQty,
		UnitPriceCents:        input.UnitPriceCents,
		AppliedVolumeDiscount: input.AppliedVolumeDiscount,
		LineSubtotalCents:     lineTotal,
		Status:                input.Status,
		Warnings:              input.Warnings,
	}
}

func selectVolumeDiscount(qty int, tiers []models.ProductVolumeDiscount) *models.ProductVolumeDiscount {
	var selected *models.ProductVolumeDiscount
	for _, tier := range tiers {
		if tier.MinQty <= qty {
			if selected == nil || tier.MinQty > selected.MinQty {
				copy := tier
				selected = &copy
			}
		}
	}
	return selected
}

func normalizeState(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
