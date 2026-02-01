package models

import "github.com/google/uuid"

// CheckoutGroup captures the persisted checkout attempt's aggregated orders and vendor snapshots.
type CheckoutGroup struct {
	ID               uuid.UUID
	BuyerStoreID     uuid.UUID
	CartID           *uuid.UUID
	VendorOrders     []VendorOrder     `gorm:"-"`
	CartVendorGroups []CartVendorGroup `gorm:"-"`
}
