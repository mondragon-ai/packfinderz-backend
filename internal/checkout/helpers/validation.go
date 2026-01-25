package helpers

import (
	"strings"

	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/checkout"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

// ValidateBuyerStore ensures the store is a verified buyer and returns its normalized state.
func ValidateBuyerStore(store *stores.StoreDTO) (string, error) {
	if store == nil {
		return "", pkgerrors.New(pkgerrors.CodeValidation, "buyer store is required")
	}
	if store.Type != enums.StoreTypeBuyer {
		return "", pkgerrors.New(pkgerrors.CodeForbidden, "active store must be a buyer")
	}
	if store.KYCStatus != enums.KYCStatusVerified {
		return "", pkgerrors.New(pkgerrors.CodeForbidden, "buyer store must be verified")
	}
	state := normalizeState(store.Address.State)
	if state == "" {
		return "", pkgerrors.New(pkgerrors.CodeValidation, "buyer store state is required")
	}
	return state, nil
}

// ValidateVendorStore confirms the vendor is active, matched to the buyer's state, and eligible for checkout.
func ValidateVendorStore(vendor *stores.StoreDTO, buyerState string) error {
	if vendor == nil {
		return pkgerrors.New(pkgerrors.CodeNotFound, "vendor not found")
	}
	if vendor.Type != enums.StoreTypeVendor {
		return pkgerrors.New(pkgerrors.CodeNotFound, "vendor not found")
	}
	if vendor.KYCStatus != enums.KYCStatusVerified {
		return pkgerrors.New(pkgerrors.CodeNotFound, "vendor not verified")
	}
	if !vendor.SubscriptionActive {
		return pkgerrors.New(pkgerrors.CodeForbidden, "vendor subscription inactive")
	}
	vendorState := normalizeState(vendor.Address.State)
	if vendorState == "" {
		return pkgerrors.New(pkgerrors.CodeValidation, "vendor state unavailable")
	}
	if buyerState != vendorState {
		return pkgerrors.New(pkgerrors.CodeValidation, "buyer store and vendor are in different states")
	}
	return nil
}

// ValidateMOQ enforces minimum order quantity compliance.
func ValidateMOQ(items []checkout.MOQValidationInput) error {
	return checkout.ValidateMOQ(items)
}

func normalizeState(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
