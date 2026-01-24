package visibility

import (
	"fmt"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

// VendorVisibilityInput drives the shared visibility checks for buyer-facing queries.
type VendorVisibilityInput struct {
	Store          *models.Store
	RequestedState string
	BuyerState     string
}

// EnsureVendorVisible enforces canonical rules so gated vendors never leak through buyer queries.
func EnsureVendorVisible(input VendorVisibilityInput) error {
	if strings.TrimSpace(input.RequestedState) == "" {
		return pkgerrors.New(pkgerrors.CodeValidation, "state is required")
	}
	if input.Store == nil {
		return pkgerrors.New(pkgerrors.CodeNotFound, "vendor not found")
	}
	if input.Store.Type != enums.StoreTypeVendor {
		return pkgerrors.New(pkgerrors.CodeNotFound, "vendor not found")
	}
	if input.Store.KYCStatus != enums.KYCStatusVerified {
		return pkgerrors.New(pkgerrors.CodeNotFound, "vendor not verified")
	}
	if !input.Store.SubscriptionActive {
		return pkgerrors.New(pkgerrors.CodeNotFound, "vendor subscription inactive")
	}
	storeState := normalizeState(input.Store.Address.State)
	if storeState == "" {
		return pkgerrors.New(pkgerrors.CodeNotFound, "vendor state unavailable")
	}

	requestedState := normalizeState(input.RequestedState)
	if storeState != requestedState {
		return pkgerrors.New(pkgerrors.CodeNotFound, "vendor not available in the requested state")
	}

	if strings.TrimSpace(input.BuyerState) != "" {
		buyerState := normalizeState(input.BuyerState)
		if buyerState != requestedState {
			return pkgerrors.New(pkgerrors.CodeValidation, fmt.Sprintf("buyer store is in %s and cannot browse %s", buyerState, requestedState))
		}
	}

	return nil
}

func normalizeState(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
