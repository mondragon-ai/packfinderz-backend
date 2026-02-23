package models

import (
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
)

// CheckoutGroup captures the persisted checkout attempt's aggregated orders and vendor snapshots.
type CheckoutGroup struct {
	ID               uuid.UUID
	BuyerStoreID     uuid.UUID
	CartID           *uuid.UUID
	BillingAddress   *types.Address
	Tip              int
	VendorOrders     []VendorOrder     `gorm:"-"`
	CartVendorGroups []CartVendorGroup `gorm:"-"`
}
