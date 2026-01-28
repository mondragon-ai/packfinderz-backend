package helpers

import (
	"strings"

	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/checkout"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/visibility"
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
	normalizedBuyer := normalizeState(buyerState)
	if normalizedBuyer == "" {
		return pkgerrors.New(pkgerrors.CodeValidation, "buyer state is required")
	}

	storeModel := convertToModel(vendor)
	return visibility.EnsureVendorVisible(visibility.VendorVisibilityInput{
		Store:          storeModel,
		RequestedState: normalizedBuyer,
		BuyerState:     normalizedBuyer,
	})
}

// ValidateMOQ enforces minimum order quantity compliance.
func ValidateMOQ(items []checkout.MOQValidationInput) error {
	return checkout.ValidateMOQ(items)
}

func normalizeState(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func convertToModel(dto *stores.StoreDTO) *models.Store {
	if dto == nil {
		return nil
	}
	return &models.Store{
		ID:                 dto.ID,
		Type:               dto.Type,
		CompanyName:        dto.CompanyName,
		KYCStatus:          dto.KYCStatus,
		SubscriptionActive: dto.SubscriptionActive,
		Address:            dto.Address,
	}
}
