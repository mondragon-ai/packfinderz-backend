package visibility

import (
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

func baseVendorStore() *models.Store {
	return &models.Store{
		Type:               enums.StoreTypeVendor,
		KYCStatus:          enums.KYCStatusVerified,
		SubscriptionActive: true,
		Address: types.Address{
			Line1:      "123 Test St",
			City:       "Testville",
			State:      "OK",
			PostalCode: "73000",
			Country:    "US",
			Lat:        0,
			Lng:        0,
		},
	}
}

func TestEnsureVendorVisible(t *testing.T) {
	base := baseVendorStore()
	t.Run("state required", func(t *testing.T) {
		err := EnsureVendorVisible(VendorVisibilityInput{Store: base})
		if err == nil {
			t.Fatal("expected validation error")
		}
		if errors.As(err).Code() != errors.CodeValidation {
			t.Fatalf("expected validation code, got %s", errors.As(err).Code())
		}
	})
	t.Run("store missing", func(t *testing.T) {
		err := EnsureVendorVisible(VendorVisibilityInput{RequestedState: "OK"})
		if err == nil {
			t.Fatal("expected not found")
		}
		if errors.As(err).Code() != errors.CodeNotFound {
			t.Fatalf("expected not found code, got %s", errors.As(err).Code())
		}
	})
	t.Run("wrong type", func(t *testing.T) {
		store := baseVendorStore()
		store.Type = enums.StoreTypeBuyer
		err := EnsureVendorVisible(VendorVisibilityInput{Store: store, RequestedState: "OK"})
		if err == nil || errors.As(err).Code() != errors.CodeNotFound {
			t.Fatalf("expected not found, got %v", err)
		}
	})
	t.Run("kyc not verified", func(t *testing.T) {
		store := baseVendorStore()
		store.KYCStatus = enums.KYCStatusPendingVerification
		err := EnsureVendorVisible(VendorVisibilityInput{Store: store, RequestedState: "OK"})
		if err == nil || errors.As(err).Code() != errors.CodeNotFound {
			t.Fatalf("expected not found, got %v", err)
		}
	})
	t.Run("subscription inactive", func(t *testing.T) {
		store := baseVendorStore()
		store.SubscriptionActive = false
		err := EnsureVendorVisible(VendorVisibilityInput{Store: store, RequestedState: "OK"})
		if err == nil || errors.As(err).Code() != errors.CodeNotFound {
			t.Fatalf("expected not found, got %v", err)
		}
	})
	t.Run("missing store state", func(t *testing.T) {
		store := baseVendorStore()
		store.Address.State = ""
		err := EnsureVendorVisible(VendorVisibilityInput{Store: store, RequestedState: "OK"})
		if err == nil || errors.As(err).Code() != errors.CodeNotFound {
			t.Fatalf("expected not found, got %v", err)
		}
	})
	t.Run("state mismatch", func(t *testing.T) {
		err := EnsureVendorVisible(VendorVisibilityInput{Store: base, RequestedState: "TX"})
		if err == nil || errors.As(err).Code() != errors.CodeNotFound {
			t.Fatalf("expected not found, got %v", err)
		}
	})
	t.Run("buyer state mismatch", func(t *testing.T) {
		err := EnsureVendorVisible(VendorVisibilityInput{Store: base, RequestedState: "OK", BuyerState: "TX"})
		if err == nil || errors.As(err).Code() != errors.CodeValidation {
			t.Fatalf("expected validation error, got %v", err)
		}
	})
	t.Run("success", func(t *testing.T) {
		err := EnsureVendorVisible(VendorVisibilityInput{Store: base, RequestedState: "ok", BuyerState: "OK"})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})
}
