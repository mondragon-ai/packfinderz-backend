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
	QuoteCart(ctx context.Context, buyerStoreID uuid.UUID, input QuoteCartInput) (*models.CartRecord, error)
	GetActiveCart(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error)
}

type service struct {
	repo           CartRepository
	tx             txRunner
	store          storeLoader
	productRepo    productLoader
	promo          promoLoader
	tokenValidator attributionTokenValidator
}

// NewService builds a cart service backed by the provided stack.
func NewService(repo CartRepository, tx txRunner, store storeLoader, productRepo productLoader, promo promoLoader, tokenValidator attributionTokenValidator) (Service, error) {
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
	if promo == nil {
		return nil, fmt.Errorf("promo loader required")
	}
	if tokenValidator == nil {
		return nil, fmt.Errorf("token validator required")
	}
	return &service{
		repo:           repo,
		tx:             tx,
		store:          store,
		productRepo:    productRepo,
		promo:          promo,
		tokenValidator: tokenValidator,
	}, nil
}
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

	items := make([]models.CartItem, 0, len(pipeline.Items))
	for _, pipelineItem := range pipeline.Items {
		items = append(items, buildCartItemFromPipeline(pipelineItem))
	}

	vendorGroups := aggregateVendorGroups(pipeline)

	subtotalCents := 0
	discountsCents := 0
	totalCents := 0

	for _, group := range vendorGroups {
		subtotalCents += group.SubtotalCents
		discountsCents += group.DiscountsCents
		totalCents += group.TotalCents
	}

	if subtotalCents < 0 {
		subtotalCents = 0
	}
	if discountsCents < 0 {
		discountsCents = 0
	}
	if discountsCents > subtotalCents {
		discountsCents = subtotalCents
	}
	if totalCents < 0 {
		totalCents = 0
	}

	shippingAddress := store.Address
	validUntil := time.Now().Add(15 * time.Minute)
	currency := enums.CurrencyUSD

	adTokens := s.filterAdTokens(input.AdTokens)

	payload := cartRecordPayload{
		ShippingAddress: &shippingAddress,
		Currency:        currency,
		ValidUntil:      validUntil,
		DiscountsCents:  discountsCents,
		SubtotalCents:   subtotalCents,
		TotalCents:      totalCents,
		AdTokens:        adTokens,
		Items:           items,
		VendorGroups:    vendorGroups,
	}

	return s.persistQuote(ctx, buyerStoreID, payload)
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

func buildCartItemFromPipeline(item *quotePipelineItem) models.CartItem {
	return models.CartItem{
		ProductID:               item.Product.ID,
		VendorStoreID:           item.Product.StoreID,
		VendorStoreName:         item.VendorStore.CompanyName,
		Unit:                    item.Product.Unit,
		Quantity:                item.NormalizedQty,
		MOQ:                     item.MOQ,
		MaxQty:                  item.MaxQty,
		Title:                   item.Title,
		Thumbnail:               item.Thumbnail,
		UnitPriceCents:          item.UnitPriceCents,
		EffectiveUnitPriceCents: item.EffectiveUnitPriceCents,
		LineDiscountsCents:      item.LineDiscountsCents,
		LineTotalCents:          item.LineTotalCents,
		AppliedVolumeDiscount:   item.AppliedVolumeDiscount,
		LineSubtotalCents:       item.LineSubtotalCents,
		Status:                  item.Status,
		Warnings:                item.Warnings,
	}
}

type cartRecordPayload struct {
	ShippingAddress *types.Address
	Currency        enums.Currency
	ValidUntil      time.Time
	DiscountsCents  int
	SubtotalCents   int
	TotalCents      int
	AdTokens        []string
	Items           []models.CartItem
	VendorGroups    []models.CartVendorGroup
}

func (s *service) persistQuote(ctx context.Context, buyerStoreID uuid.UUID, payload cartRecordPayload) (*models.CartRecord, error) {
	start := time.Now()
	fmt.Printf("[cart.persistQuote] start buyer_store_id=%s items=%d vendor_groups=%d subtotal=%d discounts=%d total=%d currency=%s valid_until=%s\n",
		buyerStoreID.String(),
		len(payload.Items),
		len(payload.VendorGroups),
		payload.SubtotalCents,
		payload.DiscountsCents,
		payload.TotalCents,
		string(payload.Currency),
		payload.ValidUntil.Format(time.RFC3339),
	)

	if buyerStoreID == uuid.Nil {
		fmt.Printf("[cart.persistQuote] validation_error reason=buyer_store_id_nil\n")
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "buyer store id is required")
	}
	if len(payload.Items) == 0 {
		fmt.Printf("[cart.persistQuote] validation_error reason=items_empty\n")
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "cart must contain at least one item")
	}
	if payload.SubtotalCents < 0 || payload.TotalCents < 0 || payload.DiscountsCents < 0 {
		fmt.Printf("[cart.persistQuote] validation_error reason=negative_totals subtotal=%d discounts=%d total=%d\n",
			payload.SubtotalCents, payload.DiscountsCents, payload.TotalCents,
		)
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "cart totals must be non-negative")
	}

	currency := payload.Currency
	if !currency.IsValid() {
		fmt.Printf("[cart.persistQuote] invalid_currency defaulting currency=%s\n", string(currency))
		currency = enums.CurrencyUSD
	}

	var saved *models.CartRecord
	if err := s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		txRepo := s.repo.WithTx(tx)

		fmt.Printf("[cart.persistQuote.tx] find_active start buyer_store_id=%s\n", buyerStoreID.String())
		record, err := txRepo.FindActiveByBuyerStore(ctx, buyerStoreID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Printf("[cart.persistQuote.tx] find_active error=%v buyer_store_id=%s\n", err, buyerStoreID.String())
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Printf("[cart.persistQuote.tx] find_active not_found buyer_store_id=%s\n", buyerStoreID.String())
		} else if record != nil {
			fmt.Printf("[cart.persistQuote.tx] find_active ok cart_id=%s buyer_store_id=%s existing_items=%d existing_vendor_groups=%d\n",
				record.ID.String(), buyerStoreID.String(), len(record.Items), len(record.VendorGroups),
			)
		}

		var targetID uuid.UUID
		if record != nil && record.ID != uuid.Nil {
			fmt.Printf("[cart.persistQuote.tx] update_cart start cart_id=%s buyer_store_id=%s\n", record.ID.String(), buyerStoreID.String())
			record.ShippingAddress = payload.ShippingAddress
			record.ValidUntil = payload.ValidUntil
			record.Currency = currency
			record.DiscountsCents = payload.DiscountsCents
			record.SubtotalCents = payload.SubtotalCents
			record.TotalCents = payload.TotalCents
			record.AdTokens = pq.StringArray(payload.AdTokens)

			if _, err := txRepo.Update(ctx, record); err != nil {
				fmt.Printf("[cart.persistQuote.tx] update_cart error=%v cart_id=%s\n", err, record.ID.String())
				return err
			}
			fmt.Printf("[cart.persistQuote.tx] update_cart ok cart_id=%s\n", record.ID.String())
			targetID = record.ID
		} else {
			fmt.Printf("[cart.persistQuote.tx] create_cart start buyer_store_id=%s\n", buyerStoreID.String())
			record = &models.CartRecord{
				BuyerStoreID:    buyerStoreID,
				ShippingAddress: payload.ShippingAddress,
				Currency:        currency,
				ValidUntil:      payload.ValidUntil,
				SubtotalCents:   payload.SubtotalCents,
				DiscountsCents:  payload.DiscountsCents,
				TotalCents:      payload.TotalCents,
				AdTokens:        pq.StringArray(payload.AdTokens),
			}
			created, err := txRepo.Create(ctx, record)
			if err != nil {
				fmt.Printf("[cart.persistQuote.tx] create_cart error=%v buyer_store_id=%s\n", err, buyerStoreID.String())
				return err
			}
			targetID = created.ID
			fmt.Printf("[cart.persistQuote.tx] create_cart ok cart_id=%s buyer_store_id=%s\n", targetID.String(), buyerStoreID.String())
		}

		for i := range payload.Items {
			payload.Items[i].CartID = targetID
		}
		for i := range payload.VendorGroups {
			payload.VendorGroups[i].CartID = targetID
		}

		fmt.Printf("[cart.persistQuote.tx] replace_items start cart_id=%s items=%d\n", targetID.String(), len(payload.Items))
		if err := txRepo.ReplaceItems(ctx, targetID, payload.Items); err != nil {
			fmt.Printf("[cart.persistQuote.tx] replace_items error=%v cart_id=%s items=%d\n", err, targetID.String(), len(payload.Items))
			return err
		}
		fmt.Printf("[cart.persistQuote.tx] replace_items ok cart_id=%s\n", targetID.String())

		fmt.Printf("[cart.persistQuote.tx] replace_vendor_groups start cart_id=%s vendor_groups=%d\n", targetID.String(), len(payload.VendorGroups))
		if err := txRepo.ReplaceVendorGroups(ctx, targetID, payload.VendorGroups); err != nil {
			fmt.Printf("[cart.persistQuote.tx] replace_vendor_groups error=%v cart_id=%s vendor_groups=%d\n", err, targetID.String(), len(payload.VendorGroups))
			return err
		}
		fmt.Printf("[cart.persistQuote.tx] replace_vendor_groups ok cart_id=%s\n", targetID.String())

		fmt.Printf("[cart.persistQuote.tx] find_saved start cart_id=%s buyer_store_id=%s\n", targetID.String(), buyerStoreID.String())
		saved, err = txRepo.FindByIDAndBuyerStore(ctx, targetID, buyerStoreID)
		if err != nil {
			fmt.Printf("[cart.persistQuote.tx] find_saved error=%v cart_id=%s buyer_store_id=%s\n", err, targetID.String(), buyerStoreID.String())
			return err
		}
		fmt.Printf("[cart.persistQuote.tx] find_saved ok cart_id=%s items=%d vendor_groups=%d\n",
			saved.ID.String(), len(saved.Items), len(saved.VendorGroups),
		)

		return nil
	}); err != nil {
		fmt.Printf("[cart.persistQuote] tx failed error=%v buyer_store_id=%s duration_ms=%d\n",
			err, buyerStoreID.String(), time.Since(start).Milliseconds(),
		)
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "persist cart")
	}

	fmt.Printf("[cart.persistQuote] done ok buyer_store_id=%s cart_id=%s duration_ms=%d\n",
		buyerStoreID.String(), saved.ID.String(), time.Since(start).Milliseconds(),
	)
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

func (s *service) filterAdTokens(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}
	var valid []string
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if !s.tokenValidator.Validate(token) {
			continue
		}
		valid = append(valid, token)
	}
	if len(valid) == 0 {
		return nil
	}
	return valid
}
