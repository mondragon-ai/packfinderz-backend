package cart

import (
	"context"
	"errors"
	"fmt"
	"strings"

	checkouthelpers "github.com/angelmondragon/packfinderz-backend/internal/checkout/helpers"
	product "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/checkout"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var validCartLevelDiscountTypes = map[string]struct{}{
	"percentage": {},
	"fixed":      {},
}

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
	SessionID          *string
	ShippingAddress    *types.Address
	TotalDiscount      int
	Fees               int
	SubtotalCents      int
	TotalCents         int
	CartLevelDiscounts types.CartLevelDiscounts
	Items              []CartItemInput
}

// CartItemInput mirrors the data stored for each cart item.
type CartItemInput struct {
	ProductID                       uuid.UUID
	VendorStoreID                   uuid.UUID
	Qty                             int
	ProductSKU                      string
	Unit                            enums.ProductUnit
	UnitPriceCents                  int
	CompareAtUnitPriceCents         *int
	AppliedVolumeTierMinQty         *int
	AppliedVolumeTierUnitPriceCents *int
	DiscountedPrice                 *int
	SubTotalPrice                   *int
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
	if input.SubtotalCents < 0 || input.TotalCents < 0 || input.TotalDiscount < 0 || input.Fees < 0 {
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

		vendor, err := s.ensureVendor(ctx, payload.VendorStoreID, buyerState, vendorCache)
		if err != nil {
			return nil, err
		}
		_ = vendor // ensure we touched vendor to avoid unused

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

		if payload.Qty > 0 && product.Inventory.AvailableQty < payload.Qty {
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

		items = append(items, buildCartItem(payload, product, linePrice, lineTotal))
	}

	if err := checkout.ValidateMOQ(moqInputs); err != nil {
		return nil, err
	}

	if err := verifyTotals(input, subtotalSum); err != nil {
		return nil, err
	}

	if err := validateCartLevelDiscounts(input.CartLevelDiscounts, vendorIDs); err != nil {
		return nil, err
	}

	var saved *models.CartRecord
	if err := s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		txRepo := s.repo.WithTx(tx)
		record, err := txRepo.FindActiveByBuyerStore(ctx, buyerStoreID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var targetID uuid.UUID
		if record != nil {
			record.SessionID = input.SessionID
			record.ShippingAddress = input.ShippingAddress
			record.TotalDiscount = input.TotalDiscount
			record.Fees = input.Fees
			record.SubtotalCents = input.SubtotalCents
			record.TotalCents = input.TotalCents
			record.CartLevelDiscounts = input.CartLevelDiscounts
			if _, err := txRepo.Update(ctx, record); err != nil {
				return err
			}
			targetID = record.ID
		} else {
			record = &models.CartRecord{
				BuyerStoreID:       buyerStoreID,
				SessionID:          input.SessionID,
				ShippingAddress:    input.ShippingAddress,
				TotalDiscount:      input.TotalDiscount,
				Fees:               input.Fees,
				SubtotalCents:      input.SubtotalCents,
				TotalCents:         input.TotalCents,
				CartLevelDiscounts: input.CartLevelDiscounts,
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
	if input.TotalDiscount > input.SubtotalCents {
		return pkgerrors.New(pkgerrors.CodeValidation, "total discount exceeds subtotal")
	}
	if input.SubtotalCents+input.Fees-input.TotalDiscount != input.TotalCents {
		return pkgerrors.New(pkgerrors.CodeValidation, "cart total mismatch")
	}
	return nil
}

func validateCartLevelDiscounts(levels types.CartLevelDiscounts, vendorIDs map[uuid.UUID]struct{}) error {
	for _, entry := range levels {
		valueType := strings.ToLower(strings.TrimSpace(entry.ValueType))
		if _, ok := validCartLevelDiscountTypes[valueType]; !ok {
			return pkgerrors.New(pkgerrors.CodeValidation, "invalid cart level discount type")
		}
		if strings.TrimSpace(entry.Title) == "" {
			return pkgerrors.New(pkgerrors.CodeValidation, "discount title is required")
		}
		if entry.ID == uuid.Nil {
			return pkgerrors.New(pkgerrors.CodeValidation, "discount id is required")
		}
		if strings.TrimSpace(entry.Value) == "" {
			return pkgerrors.New(pkgerrors.CodeValidation, "discount value is required")
		}
		if entry.VendorID == uuid.Nil {
			return pkgerrors.New(pkgerrors.CodeValidation, "discount vendor is required")
		}
		if _, ok := vendorIDs[entry.VendorID]; !ok {
			return pkgerrors.New(pkgerrors.CodeValidation, "discount references unknown vendor")
		}
	}
	return nil
}

func buildCartItem(input CartItemInput, product *models.Product, discountedPrice, lineTotal int) models.CartItem {
	return models.CartItem{
		ProductID:                       product.ID,
		VendorStoreID:                   product.StoreID,
		Qty:                             input.Qty,
		ProductSKU:                      product.SKU,
		Unit:                            input.Unit,
		UnitPriceCents:                  input.UnitPriceCents,
		CompareAtUnitPriceCents:         copyIntPtr(input.CompareAtUnitPriceCents, product.CompareAtPriceCents),
		AppliedVolumeTierMinQty:         copyIntPtr(input.AppliedVolumeTierMinQty),
		AppliedVolumeTierUnitPriceCents: copyIntPtr(input.AppliedVolumeTierUnitPriceCents),
		DiscountedPrice:                 intPtr(discountedPrice),
		SubTotalPrice:                   intPtr(lineTotal),
		FeaturedImage:                   input.FeaturedImage,
		MOQ:                             intPtr(product.MOQ),
		THCPercent:                      copyFloatPtr(input.THCPercent),
		CBDPercent:                      copyFloatPtr(input.CBDPercent),
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

func intPtr(value int) *int {
	return &value
}

func copyIntPtr(src *int, fallback ...*int) *int {
	if src != nil {
		val := *src
		return &val
	}
	for _, ptr := range fallback {
		if ptr != nil {
			val := *ptr
			return &val
		}
	}
	return nil
}

func copyFloatPtr(src *float64) *float64 {
	if src == nil {
		return nil
	}
	val := *src
	return &val
}
