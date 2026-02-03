package cart

import (
	"context"
	"fmt"
	"testing"

	products "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestSelectVolumeDiscount(t *testing.T) {
	t.Parallel()

	tiers := []models.ProductVolumeDiscount{
		{MinQty: 10, DiscountPercent: 20},
		{MinQty: 5, DiscountPercent: 10},
		{MinQty: 20, DiscountPercent: 30},
	}

	if res := selectVolumeDiscount(12, tiers); res == nil || res.MinQty != 10 {
		t.Fatalf("expected tier with min qty 10, got %+v", res)
	}

	if res := selectVolumeDiscount(4, tiers); res != nil {
		t.Fatalf("expected no tier for qty 4, got %+v", res)
	}

	if res := selectVolumeDiscount(25, tiers); res == nil || res.MinQty != 20 {
		t.Fatalf("expected highest tier for qty 25, got %+v", res)
	}
}
func (s *stubCartRepo) UpdateStatus(ctx context.Context, id, buyerStoreID uuid.UUID, status enums.CartStatus) error {
	return nil
}

func TestServiceGetActiveCartNotFound(t *testing.T) {
	t.Parallel()

	store := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US", Lat: 0, Lng: 0},
	}
	repo := &stubCartRepo{findErr: gorm.ErrRecordNotFound}
	svc := newTestService(repo, store)

	_, err := svc.GetActiveCart(context.Background(), store.ID)
	if err == nil {
		t.Fatal("expected error for missing cart")
	}
	if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeNotFound {
		t.Fatalf("unexpected error code: %v", err)
	}
}

func TestServiceGetActiveCartSuccess(t *testing.T) {
	t.Parallel()

	store := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US", Lat: 0, Lng: 0},
	}
	record := &models.CartRecord{ID: uuid.New(), BuyerStoreID: store.ID, Status: enums.CartStatusActive}
	repo := &stubCartRepo{record: record}
	svc := newTestService(repo, store)

	got, err := svc.GetActiveCart(context.Background(), store.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != record {
		t.Fatalf("expected record to match")
	}
}

func newTestService(repo CartRepository, store *stores.StoreDTO) Service {
	svc, err := NewService(repo, stubTxRunner{}, storeLoaderFunc(func(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error) {
		return store, nil
	}), stubProductLoader{products: map[uuid.UUID]*models.Product{}}, NoopPromoLoader(), stubTokenValidator{})
	if err != nil {
		panic(err)
	}
	return svc
}

func TestQuoteCartIgnoresInventoryShortage(t *testing.T) {
	t.Parallel()

	buyerStore := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	vendorID := uuid.New()
	vendorStore := &stores.StoreDTO{
		ID:                 vendorID,
		Type:               enums.StoreTypeVendor,
		KYCStatus:          enums.KYCStatusVerified,
		SubscriptionActive: true,
		Address:            types.Address{Line1: "2", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	productID := uuid.New()
	product := &models.Product{
		ID:         productID,
		StoreID:    vendorID,
		SKU:        "SKU",
		Unit:       enums.ProductUnitUnit,
		MOQ:        1,
		PriceCents: 1000,
		IsActive:   true,
		Inventory: &models.InventoryItem{
			ProductID:    productID,
			AvailableQty: 0,
		},
	}
	repo := &stubCartRepo{}
	service, err := NewService(repo, stubTxRunner{}, storeLoaderFunc(func(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error) {
		switch id {
		case buyerStore.ID:
			return buyerStore, nil
		case vendorStore.ID:
			return vendorStore, nil
		default:
			return nil, fmt.Errorf("store %s not found", id)
		}
	}), stubProductLoader{products: map[uuid.UUID]*models.Product{product.ID: product}}, NoopPromoLoader(), stubTokenValidator{})
	if err != nil {
		t.Fatalf("failed to build service: %v", err)
	}

	input := QuoteCartInput{
		Items: []QuoteCartItem{{
			ProductID:     product.ID,
			VendorStoreID: vendorID,
			Quantity:      5,
		}},
	}

	if _, err := service.QuoteCart(context.Background(), buyerStore.ID, input); err != nil {
		t.Fatalf("expected quote to ignore inventory shortage, got %v", err)
	}
}

func TestQuoteCartVendorPreloadsOncePerVendor(t *testing.T) {
	t.Parallel()

	buyerStore := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	vendor1 := &stores.StoreDTO{
		ID:                 uuid.New(),
		Type:               enums.StoreTypeVendor,
		KYCStatus:          enums.KYCStatusVerified,
		SubscriptionActive: true,
		Address:            types.Address{Line1: "2", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	vendor2 := &stores.StoreDTO{
		ID:                 uuid.New(),
		Type:               enums.StoreTypeVendor,
		KYCStatus:          enums.KYCStatusVerified,
		SubscriptionActive: true,
		Address:            types.Address{Line1: "3", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}

	product1 := &models.Product{
		ID:         uuid.New(),
		StoreID:    vendor1.ID,
		SKU:        "SKU1",
		Unit:       enums.ProductUnitUnit,
		MOQ:        1,
		PriceCents: 1000,
		IsActive:   true,
	}
	product2 := &models.Product{
		ID:         uuid.New(),
		StoreID:    vendor2.ID,
		SKU:        "SKU2",
		Unit:       enums.ProductUnitUnit,
		MOQ:        1,
		PriceCents: 1200,
		IsActive:   true,
	}
	products := map[uuid.UUID]*models.Product{
		product1.ID: product1,
		product2.ID: product2,
	}

	loader := newCountingStoreLoader(map[uuid.UUID]*stores.StoreDTO{
		buyerStore.ID: buyerStore,
		vendor1.ID:    vendor1,
		vendor2.ID:    vendor2,
	})

	repo := &stubCartRepo{}
	service, err := NewService(repo, stubTxRunner{}, loader, stubProductLoader{products: products}, NoopPromoLoader(), stubTokenValidator{})
	if err != nil {
		t.Fatalf("failed to build service: %v", err)
	}

	input := QuoteCartInput{
		Items: []QuoteCartItem{
			{ProductID: product1.ID, VendorStoreID: vendor1.ID, Quantity: 1},
			{ProductID: product1.ID, VendorStoreID: vendor1.ID, Quantity: 2},
			{ProductID: product2.ID, VendorStoreID: vendor2.ID, Quantity: 1},
		},
	}

	if _, err := service.QuoteCart(context.Background(), buyerStore.ID, input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loader.calls[vendor1.ID] != 1 {
		t.Fatalf("expected vendor1 resolved once, got %d", loader.calls[vendor1.ID])
	}
	if loader.calls[vendor2.ID] != 1 {
		t.Fatalf("expected vendor2 resolved once, got %d", loader.calls[vendor2.ID])
	}
}

func TestQuoteCartFiltersInvalidAdTokens(t *testing.T) {
	t.Parallel()

	buyerStore := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	vendorStore := &stores.StoreDTO{
		ID:                 uuid.New(),
		Type:               enums.StoreTypeVendor,
		KYCStatus:          enums.KYCStatusVerified,
		SubscriptionActive: true,
		Address:            types.Address{Line1: "2", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	productID := uuid.New()
	product := &models.Product{
		ID:         productID,
		StoreID:    vendorStore.ID,
		SKU:        "SKU",
		Unit:       enums.ProductUnitUnit,
		MOQ:        1,
		PriceCents: 1000,
		IsActive:   true,
	}

	loader := newCountingStoreLoader(map[uuid.UUID]*stores.StoreDTO{
		buyerStore.ID:  buyerStore,
		vendorStore.ID: vendorStore,
	})

	repo := &stubCartRepo{}
	validator := stubTokenValidator{validTokens: map[string]struct{}{
		"token-valid": {},
	}}
	service, err := NewService(repo, stubTxRunner{}, loader, stubProductLoader{products: map[uuid.UUID]*models.Product{product.ID: product}}, NoopPromoLoader(), validator)
	if err != nil {
		t.Fatalf("failed to build service: %v", err)
	}

	input := QuoteCartInput{
		Items: []QuoteCartItem{{
			ProductID:     product.ID,
			VendorStoreID: vendorStore.ID,
			Quantity:      1,
		}},
		AdTokens: []string{"token-valid", "token-invalid"},
	}

	record, err := service.QuoteCart(context.Background(), buyerStore.ID, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotTokens := []string(record.AdTokens)
	if len(gotTokens) != 1 || gotTokens[0] != "token-valid" {
		t.Fatalf("expected only valid tokens persisted, got %+v", gotTokens)
	}
}

func TestQuoteCartDetectsProductVendorMismatch(t *testing.T) {
	t.Parallel()

	buyerStore := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	vendorID := uuid.New()
	vendorStore := &stores.StoreDTO{
		ID:                 vendorID,
		Type:               enums.StoreTypeVendor,
		KYCStatus:          enums.KYCStatusVerified,
		SubscriptionActive: true,
		Address:            types.Address{Line1: "2", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	otherProduct := &models.Product{
		ID:         uuid.New(),
		StoreID:    uuid.New(), // different store
		SKU:        "SKU",
		Unit:       enums.ProductUnitUnit,
		MOQ:        1,
		PriceCents: 1000,
		IsActive:   true,
	}

	loader := newCountingStoreLoader(map[uuid.UUID]*stores.StoreDTO{
		buyerStore.ID:  buyerStore,
		vendorStore.ID: vendorStore,
	})

	repo := &stubCartRepo{}
	service, err := NewService(repo, stubTxRunner{}, loader, stubProductLoader{products: map[uuid.UUID]*models.Product{otherProduct.ID: otherProduct}}, NoopPromoLoader(), stubTokenValidator{})
	if err != nil {
		t.Fatalf("failed to build service: %v", err)
	}

	input := QuoteCartInput{
		Items: []QuoteCartItem{{
			ProductID:     otherProduct.ID,
			VendorStoreID: vendorStore.ID,
			Quantity:      1,
		}},
	}

	record, err := service.QuoteCart(context.Background(), buyerStore.ID, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.replaced) != 1 {
		t.Fatalf("expected 1 item persisted, got %d", len(repo.replaced))
	}

	item := repo.replaced[0]
	if item.Status != enums.CartItemStatusInvalid {
		t.Fatalf("expected invalid status, got %s", item.Status)
	}
	if len(item.Warnings) == 0 {
		t.Fatal("expected warnings for invalid item")
	}
	if item.Warnings[0].Type != enums.CartItemWarningTypeVendorMismatch {
		t.Fatalf("unexpected warning type %s", item.Warnings[0].Type)
	}
	if record == nil {
		t.Fatal("expected record returned")
	}
}

func TestQuoteCartClampsQuantityToMOQ(t *testing.T) {
	t.Parallel()

	buyerStore := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	vendorStore := &stores.StoreDTO{
		ID:                 uuid.New(),
		Type:               enums.StoreTypeVendor,
		KYCStatus:          enums.KYCStatusVerified,
		SubscriptionActive: true,
		Address:            types.Address{Line1: "2", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	productID := uuid.New()
	product := &models.Product{
		ID:         productID,
		StoreID:    vendorStore.ID,
		SKU:        "SKU",
		Unit:       enums.ProductUnitUnit,
		MOQ:        5,
		PriceCents: 1000,
		IsActive:   true,
		Inventory: &models.InventoryItem{
			ProductID:    productID,
			AvailableQty: 10,
		},
	}

	loader := newCountingStoreLoader(map[uuid.UUID]*stores.StoreDTO{
		buyerStore.ID:  buyerStore,
		vendorStore.ID: vendorStore,
	})

	repo := &stubCartRepo{}
	service, err := NewService(repo, stubTxRunner{}, loader, stubProductLoader{products: map[uuid.UUID]*models.Product{product.ID: product}}, NoopPromoLoader(), stubTokenValidator{})
	if err != nil {
		t.Fatalf("failed to build service: %v", err)
	}

	input := QuoteCartInput{
		Items: []QuoteCartItem{{
			ProductID:     product.ID,
			VendorStoreID: vendorStore.ID,
			Quantity:      2,
		}},
	}

	if _, err := service.QuoteCart(context.Background(), buyerStore.ID, input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.replaced) != 1 {
		t.Fatalf("expected 1 item persisted, got %d", len(repo.replaced))
	}

	item := repo.replaced[0]
	if item.Quantity != product.MOQ {
		t.Fatalf("expected quantity clamped to MOQ (%d), got %d", product.MOQ, item.Quantity)
	}
	if item.MOQ != product.MOQ {
		t.Fatalf("expected saved MOQ %d, got %d", product.MOQ, item.MOQ)
	}
	if item.Status != enums.CartItemStatusOK {
		t.Fatalf("expected ok status, got %s", item.Status)
	}
	if len(item.Warnings) == 0 || item.Warnings[0].Type != enums.CartItemWarningTypeClampedToMOQ {
		t.Fatalf("expected clamp warning, got %+v", item.Warnings)
	}
}

func TestQuoteCartMarksNotAvailableWhenInventoryInsufficient(t *testing.T) {
	t.Parallel()

	buyerStore := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	vendorStore := &stores.StoreDTO{
		ID:                 uuid.New(),
		Type:               enums.StoreTypeVendor,
		KYCStatus:          enums.KYCStatusVerified,
		SubscriptionActive: true,
		Address:            types.Address{Line1: "2", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	productID := uuid.New()
	product := &models.Product{
		ID:         productID,
		StoreID:    vendorStore.ID,
		SKU:        "SKU",
		Unit:       enums.ProductUnitUnit,
		MOQ:        1,
		PriceCents: 1000,
		IsActive:   true,
		Inventory: &models.InventoryItem{
			ProductID:    productID,
			AvailableQty: 1,
		},
	}

	loader := newCountingStoreLoader(map[uuid.UUID]*stores.StoreDTO{
		buyerStore.ID:  buyerStore,
		vendorStore.ID: vendorStore,
	})

	repo := &stubCartRepo{}
	service, err := NewService(repo, stubTxRunner{}, loader, stubProductLoader{products: map[uuid.UUID]*models.Product{product.ID: product}}, NoopPromoLoader(), stubTokenValidator{})
	if err != nil {
		t.Fatalf("failed to build service: %v", err)
	}

	input := QuoteCartInput{
		Items: []QuoteCartItem{{
			ProductID:     product.ID,
			VendorStoreID: vendorStore.ID,
			Quantity:      3,
		}},
	}

	if _, err := service.QuoteCart(context.Background(), buyerStore.ID, input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.replaced) != 1 {
		t.Fatalf("expected 1 item persisted, got %d", len(repo.replaced))
	}

	item := repo.replaced[0]
	if item.Status != enums.CartItemStatusNotAvailable {
		t.Fatalf("expected not_available status, got %s", item.Status)
	}
	if len(item.Warnings) == 0 || item.Warnings[0].Type != enums.CartItemWarningTypeNotAvailable {
		t.Fatalf("expected not available warning, got %+v", item.Warnings)
	}
}

func TestQuoteCartPersistsVendorGroups(t *testing.T) {
	t.Parallel()

	buyerStore := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	vendor1 := &stores.StoreDTO{
		ID:                 uuid.New(),
		Type:               enums.StoreTypeVendor,
		KYCStatus:          enums.KYCStatusVerified,
		SubscriptionActive: true,
		Address:            types.Address{Line1: "2", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	vendor2 := &stores.StoreDTO{
		ID:                 uuid.New(),
		Type:               enums.StoreTypeVendor,
		KYCStatus:          enums.KYCStatusVerified,
		SubscriptionActive: true,
		Address:            types.Address{Line1: "3", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}

	product1 := &models.Product{
		ID:         uuid.New(),
		StoreID:    vendor1.ID,
		SKU:        "SKU1",
		Unit:       enums.ProductUnitUnit,
		MOQ:        1,
		PriceCents: 1000,
		IsActive:   true,
		Inventory: &models.InventoryItem{
			ProductID:    uuid.New(),
			AvailableQty: 10,
		},
	}
	product2 := &models.Product{
		ID:         uuid.New(),
		StoreID:    vendor2.ID,
		SKU:        "SKU2",
		Unit:       enums.ProductUnitUnit,
		MOQ:        1,
		PriceCents: 1200,
		IsActive:   true,
		Inventory: &models.InventoryItem{
			ProductID:    uuid.New(),
			AvailableQty: 0,
		},
	}

	loader := newCountingStoreLoader(map[uuid.UUID]*stores.StoreDTO{
		buyerStore.ID: buyerStore,
		vendor1.ID:    vendor1,
		vendor2.ID:    vendor2,
	})

	repo := &stubCartRepo{}
	service, err := NewService(repo, stubTxRunner{}, loader, stubProductLoader{products: map[uuid.UUID]*models.Product{
		product1.ID: product1,
		product2.ID: product2,
	}}, NoopPromoLoader(), stubTokenValidator{})
	if err != nil {
		t.Fatalf("failed to build service: %v", err)
	}

	input := QuoteCartInput{
		Items: []QuoteCartItem{
			{ProductID: product1.ID, VendorStoreID: vendor1.ID, Quantity: 1},
			{ProductID: product2.ID, VendorStoreID: vendor2.ID, Quantity: 1},
		},
	}

	if _, err := service.QuoteCart(context.Background(), buyerStore.ID, input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.replacedGroups) != 2 {
		t.Fatalf("expected 2 vendor groups, got %d", len(repo.replacedGroups))
	}

	var okGroupFound bool
	var invalidGroupFound bool
	for _, group := range repo.replacedGroups {
		if group.VendorStoreID == vendor1.ID {
			if group.Status != enums.VendorGroupStatusOK {
				t.Fatalf("expected vendor1 status ok, got %s", group.Status)
			}
			if group.SubtotalCents == 0 || group.TotalCents != group.SubtotalCents {
				t.Fatalf("unexpected vendor1 totals %+v", group)
			}
			okGroupFound = true
		}
		if group.VendorStoreID == vendor2.ID {
			if group.Status != enums.VendorGroupStatusInvalid {
				t.Fatalf("expected vendor2 status invalid, got %s", group.Status)
			}
			if len(group.Warnings) == 0 || group.Warnings[0].Type != enums.VendorGroupWarningTypeVendorInvalid {
				t.Fatalf("expected vendor2 warning, got %+v", group.Warnings)
			}
			invalidGroupFound = true
		}
	}

	if !okGroupFound || !invalidGroupFound {
		t.Fatalf("expected both vendor groups to be populated")
	}

	// Re-quoting should replace vendor groups without accumulating duplicates.
	repo.replacedGroups = nil
	if _, err := service.QuoteCart(context.Background(), buyerStore.ID, input); err != nil {
		t.Fatalf("unexpected error on re-quote: %v", err)
	}
	if len(repo.replacedGroups) != 2 {
		t.Fatalf("expected 2 vendor groups after re-quote, got %d", len(repo.replacedGroups))
	}
}

func TestQuoteCartWarnsOnInvalidVendorPromo(t *testing.T) {
	t.Parallel()

	buyerStore := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	vendorStore := &stores.StoreDTO{
		ID:                 uuid.New(),
		Type:               enums.StoreTypeVendor,
		KYCStatus:          enums.KYCStatusVerified,
		SubscriptionActive: true,
		Address:            types.Address{Line1: "2", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	productID := uuid.New()
	product := &models.Product{
		ID:         productID,
		StoreID:    vendorStore.ID,
		SKU:        "SKU",
		Unit:       enums.ProductUnitUnit,
		MOQ:        1,
		PriceCents: 1500,
		IsActive:   true,
		Inventory: &models.InventoryItem{
			ProductID:    productID,
			AvailableQty: 10,
		},
	}

	loader := newCountingStoreLoader(map[uuid.UUID]*stores.StoreDTO{
		buyerStore.ID:  buyerStore,
		vendorStore.ID: vendorStore,
	})

	repo := &stubCartRepo{}
	service, err := NewService(repo, stubTxRunner{}, loader, stubProductLoader{products: map[uuid.UUID]*models.Product{product.ID: product}}, NoopPromoLoader(), stubTokenValidator{})
	if err != nil {
		t.Fatalf("failed to build service: %v", err)
	}

	input := QuoteCartInput{
		Items: []QuoteCartItem{{
			ProductID:     product.ID,
			VendorStoreID: vendorStore.ID,
			Quantity:      1,
		}},
		VendorPromos: []QuoteVendorPromo{{
			VendorStoreID: vendorStore.ID,
			Code:          "SAVE20",
		}},
	}
	if _, err := service.QuoteCart(context.Background(), buyerStore.ID, input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.replacedGroups) != 1 {
		t.Fatalf("expected 1 vendor group, got %d", len(repo.replacedGroups))
	}
	group := repo.replacedGroups[0]
	if len(group.Warnings) == 0 {
		t.Fatalf("expected warnings for invalid promo, got %+v", group.Warnings)
	}
	if group.Warnings[0].Type != enums.VendorGroupWarningTypeInvalidPromo {
		t.Fatalf("unexpected warning type %s", group.Warnings[0].Type)
	}
	if group.Warnings[0].Message != invalidPromoWarningMessage {
		t.Fatalf("unexpected warning message %q", group.Warnings[0].Message)
	}
}

func TestQuoteCartPersistsVolumeDiscount(t *testing.T) {
	t.Parallel()

	buyerStore := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	vendorStore := &stores.StoreDTO{
		ID:                 uuid.New(),
		Type:               enums.StoreTypeVendor,
		KYCStatus:          enums.KYCStatusVerified,
		SubscriptionActive: true,
		Address:            types.Address{Line1: "2", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	productID := uuid.New()
	product := &models.Product{
		ID:         productID,
		StoreID:    vendorStore.ID,
		SKU:        "SKU",
		Unit:       enums.ProductUnitUnit,
		MOQ:        1,
		PriceCents: 1000,
		IsActive:   true,
		Inventory: &models.InventoryItem{
			ProductID:    productID,
			AvailableQty: 20,
		},
		VolumeDiscounts: []models.ProductVolumeDiscount{
			{MinQty: 5, DiscountPercent: 20},
		},
	}

	loader := newCountingStoreLoader(map[uuid.UUID]*stores.StoreDTO{
		buyerStore.ID:  buyerStore,
		vendorStore.ID: vendorStore,
	})

	repo := &stubCartRepo{}
	service, err := NewService(repo, stubTxRunner{}, loader, stubProductLoader{products: map[uuid.UUID]*models.Product{product.ID: product}}, NoopPromoLoader(), stubTokenValidator{})
	if err != nil {
		t.Fatalf("failed to build service: %v", err)
	}

	input := QuoteCartInput{
		Items: []QuoteCartItem{{
			ProductID:     product.ID,
			VendorStoreID: vendorStore.ID,
			Quantity:      5,
		}},
	}

	if _, err := service.QuoteCart(context.Background(), buyerStore.ID, input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.replaced) != 1 {
		t.Fatalf("expected 1 item persisted, got %d", len(repo.replaced))
	}

	item := repo.replaced[0]
	if item.UnitPriceCents != 800 {
		t.Fatalf("expected tier price 800, got %d", item.UnitPriceCents)
	}
	if item.AppliedVolumeDiscount == nil {
		t.Fatalf("expected applied volume discount, got nil")
	}
	expectedLabel := fmt.Sprintf("volume tier %d+", product.VolumeDiscounts[0].MinQty)
	if item.AppliedVolumeDiscount.Label != expectedLabel {
		t.Fatalf("unexpected label %s", item.AppliedVolumeDiscount.Label)
	}
	expectedAmount := (product.PriceCents - item.UnitPriceCents) * item.Quantity
	if item.AppliedVolumeDiscount.AmountCents != expectedAmount {
		t.Fatalf("unexpected discount amount %d", item.AppliedVolumeDiscount.AmountCents)
	}
	expectedLine := item.UnitPriceCents * item.Quantity
	if item.LineSubtotalCents != expectedLine {
		t.Fatalf("expected line subtotal %d, got %d", expectedLine, item.LineSubtotalCents)
	}
}

func TestQuoteCartAddsPriceChangedWarning(t *testing.T) {
	t.Parallel()

	buyerStore := &stores.StoreDTO{
		ID:        uuid.New(),
		Type:      enums.StoreTypeBuyer,
		KYCStatus: enums.KYCStatusVerified,
		Address:   types.Address{Line1: "1", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	vendorStore := &stores.StoreDTO{
		ID:                 uuid.New(),
		Type:               enums.StoreTypeVendor,
		KYCStatus:          enums.KYCStatusVerified,
		SubscriptionActive: true,
		Address:            types.Address{Line1: "2", City: "City", State: "OK", PostalCode: "00000", Country: "US"},
	}
	productID := uuid.New()
	product := &models.Product{
		ID:         productID,
		StoreID:    vendorStore.ID,
		SKU:        "SKU",
		Unit:       enums.ProductUnitUnit,
		MOQ:        1,
		PriceCents: 1000,
		IsActive:   true,
		Inventory: &models.InventoryItem{
			ProductID:    productID,
			AvailableQty: 20,
		},
		VolumeDiscounts: []models.ProductVolumeDiscount{
			{MinQty: 10, DiscountPercent: 10},
		},
	}

	loader := newCountingStoreLoader(map[uuid.UUID]*stores.StoreDTO{
		buyerStore.ID:  buyerStore,
		vendorStore.ID: vendorStore,
	})

	repo := &stubCartRepo{
		record: &models.CartRecord{
			BuyerStoreID: buyerStore.ID,
			Status:       enums.CartStatusActive,
			Items: []models.CartItem{
				{
					ProductID:      product.ID,
					VendorStoreID:  vendorStore.ID,
					UnitPriceCents: product.PriceCents,
				},
			},
		},
	}
	service, err := NewService(repo, stubTxRunner{}, loader, stubProductLoader{products: map[uuid.UUID]*models.Product{product.ID: product}}, NoopPromoLoader(), stubTokenValidator{})
	if err != nil {
		t.Fatalf("failed to build service: %v", err)
	}

	input := QuoteCartInput{
		Items: []QuoteCartItem{{
			ProductID:     product.ID,
			VendorStoreID: vendorStore.ID,
			Quantity:      10,
		}},
	}

	if _, err := service.QuoteCart(context.Background(), buyerStore.ID, input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.replaced) != 1 {
		t.Fatalf("expected 1 item persisted, got %d", len(repo.replaced))
	}

	item := repo.replaced[0]
	found := false
	for _, warning := range item.Warnings {
		if warning.Type == enums.CartItemWarningTypePriceChanged {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected price_changed warning, got %+v", item.Warnings)
	}
}

type stubCartRepo struct {
	record         *models.CartRecord
	findErr        error
	replaced       []models.CartItem
	replacedGroups []models.CartVendorGroup
}

func (s *stubCartRepo) WithTx(tx *gorm.DB) CartRepository { return s }
func (s *stubCartRepo) FindActiveByBuyerStore(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.record == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.record, nil
}
func (s *stubCartRepo) FindByIDAndBuyerStore(ctx context.Context, id, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	if s.record == nil {
		return nil, gorm.ErrRecordNotFound
	}
	s.record.Items = append([]models.CartItem(nil), s.replaced...)
	return s.record, nil
}
func (s *stubCartRepo) Create(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	s.record = record
	return record, nil
}
func (s *stubCartRepo) Update(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	s.record = record
	return record, nil
}
func (s *stubCartRepo) ReplaceItems(ctx context.Context, cartID uuid.UUID, items []models.CartItem) error {
	s.replaced = append([]models.CartItem(nil), items...)
	return nil
}

func (s *stubCartRepo) ReplaceVendorGroups(ctx context.Context, cartID uuid.UUID, groups []models.CartVendorGroup) error {
	s.replacedGroups = append([]models.CartVendorGroup(nil), groups...)
	return nil
}

type stubTxRunner struct{}

func (stubTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

type storeLoaderFunc func(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error)

func (fn storeLoaderFunc) GetByID(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error) {
	return fn(ctx, id)
}

type countingStoreLoader struct {
	stores map[uuid.UUID]*stores.StoreDTO
	calls  map[uuid.UUID]int
}

func newCountingStoreLoader(stores map[uuid.UUID]*stores.StoreDTO) *countingStoreLoader {
	return &countingStoreLoader{
		stores: stores,
		calls:  make(map[uuid.UUID]int),
	}
}

func (l *countingStoreLoader) GetByID(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error) {
	l.calls[id]++
	if store, ok := l.stores[id]; ok {
		return store, nil
	}
	return nil, fmt.Errorf("store %s not found", id)
}

type stubTokenValidator struct {
	validTokens map[string]struct{}
}

func (s stubTokenValidator) Validate(token string) bool {
	if len(s.validTokens) == 0 {
		return true
	}
	_, ok := s.validTokens[token]
	return ok
}

type stubProductLoader struct {
	products map[uuid.UUID]*models.Product
	err      error
}

func (s stubProductLoader) GetProductDetail(ctx context.Context, id uuid.UUID) (*models.Product, *products.VendorSummary, error) {
	if s.err != nil {
		return nil, nil, s.err
	}
	if product, ok := s.products[id]; ok {
		return product, nil, nil
	}
	return nil, nil, gorm.ErrRecordNotFound
}
