package helpers

import (
	"testing"

	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/checkout"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
)

func TestGroupCartItemsByVendor(t *testing.T) {
	t.Parallel()
	vendorA := uuid.New()
	vendorB := uuid.New()
	items := []models.CartItem{
		{ID: uuid.New(), VendorStoreID: vendorA},
		{ID: uuid.New(), VendorStoreID: vendorB},
		{ID: uuid.New(), VendorStoreID: vendorA},
	}

	grouped := GroupCartItemsByVendor(items)
	if len(grouped) != 2 {
		t.Fatalf("expected 2 vendors, got %d", len(grouped))
	}
	if len(grouped[vendorA]) != 2 {
		t.Fatalf("expected 2 items for vendorA, got %d", len(grouped[vendorA]))
	}
	if len(grouped[vendorB]) != 1 {
		t.Fatalf("expected 1 item for vendorB, got %d", len(grouped[vendorB]))
	}
}

func TestComputeVendorTotals(t *testing.T) {
	t.Parallel()
	vendor := uuid.New()
	items := []models.CartItem{
		{
			VendorStoreID:     vendor,
			UnitPriceCents:    1000,
			Quantity:          2,
			LineSubtotalCents: 1800,
		},
		{
			VendorStoreID:     vendor,
			UnitPriceCents:    500,
			Quantity:          1,
			LineSubtotalCents: 500,
		},
	}

	totals := ComputeVendorTotals(items)
	if totals.SubtotalCents != 2500 {
		t.Fatalf("expected subtotal 2500, got %d", totals.SubtotalCents)
	}
	if totals.TotalCents != 2300 {
		t.Fatalf("expected total 2300, got %d", totals.TotalCents)
	}
	if totals.DiscountsCents != 200 {
		t.Fatalf("expected discount 200, got %d", totals.DiscountsCents)
	}
	if totals.ItemCount != 2 {
		t.Fatalf("expected item count 2, got %d", totals.ItemCount)
	}

	byVendor := ComputeTotalsByVendor(items)
	if got, ok := byVendor[vendor]; !ok {
		t.Fatalf("expected vendor totals present")
	} else if got.TotalCents != totals.TotalCents {
		t.Fatalf("expected matching totals, got %d vs %d", got.TotalCents, totals.TotalCents)
	}
}

func TestValidateBuyerStore(t *testing.T) {
	t.Parallel()
	store := newStore(enums.StoreTypeBuyer, enums.KYCStatusVerified, "ok")
	state, err := ValidateBuyerStore(store)
	if err != nil {
		t.Fatalf("expected buyer validation to pass, got %v", err)
	}
	if state != "OK" {
		t.Fatalf("expected normalized state OK, got %s", state)
	}

	store.Type = enums.StoreTypeVendor
	if _, err := ValidateBuyerStore(store); err == nil {
		t.Fatal("expected error for non-buyer store")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeForbidden {
		t.Fatalf("unexpected error code: %v", err)
	}

	store.Type = enums.StoreTypeBuyer
	store.KYCStatus = enums.KYCStatusPendingVerification
	if _, err := ValidateBuyerStore(store); err == nil {
		t.Fatal("expected error for unverified buyer")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeForbidden {
		t.Fatalf("unexpected error: %v", err)
	}

	store.KYCStatus = enums.KYCStatusVerified
	store.Address.State = ""
	if _, err := ValidateBuyerStore(store); err == nil {
		t.Fatal("expected error for missing state")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeValidation {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateVendorStore(t *testing.T) {
	t.Parallel()
	vendor := newStore(enums.StoreTypeVendor, enums.KYCStatusVerified, "OK")
	vendor.SubscriptionActive = true
	if err := ValidateVendorStore(vendor, "OK"); err != nil {
		t.Fatalf("expected vendor validation to pass, got %v", err)
	}

	vendor.Type = enums.StoreTypeBuyer
	if err := ValidateVendorStore(vendor, "OK"); err == nil {
		t.Fatal("expected error for wrong store type")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeNotFound {
		t.Fatalf("unexpected error: %v", err)
	}

	vendor.Type = enums.StoreTypeVendor
	vendor.KYCStatus = enums.KYCStatusPendingVerification
	if err := ValidateVendorStore(vendor, "OK"); err == nil {
		t.Fatal("expected error for unverified vendor")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeNotFound {
		t.Fatalf("unexpected error: %v", err)
	}

	vendor.KYCStatus = enums.KYCStatusVerified
	vendor.SubscriptionActive = false
	if err := ValidateVendorStore(vendor, "OK"); err == nil {
		t.Fatal("expected error for inactive subscription")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeNotFound {
		t.Fatalf("unexpected error: %v", err)
	}

	vendor.SubscriptionActive = true
	vendor.Address.State = "TX"
	if err := ValidateVendorStore(vendor, "OK"); err == nil {
		t.Fatal("expected error for state mismatch")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeNotFound {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateMOQ(t *testing.T) {
	t.Parallel()
	items := []checkout.MOQValidationInput{
		{ProductID: uuid.New(), ProductName: "test", MOQ: 5, Quantity: 3},
	}
	if err := ValidateMOQ(items); err == nil {
		t.Fatal("expected MOQ violation")
	} else if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeStateConflict {
		t.Fatalf("unexpected error: %v", err)
	}

	items[0].Quantity = 5
	if err := ValidateMOQ(items); err != nil {
		t.Fatalf("expected MOQ to pass, got %v", err)
	}
}

func newStore(storeType enums.StoreType, status enums.KYCStatus, state string) *stores.StoreDTO {
	return &stores.StoreDTO{
		ID:                 uuid.New(),
		Type:               storeType,
		KYCStatus:          status,
		SubscriptionActive: true,
		Address: types.Address{
			Line1: "123 Main St",
			City:  "City",
			State: state,
		},
	}
}
