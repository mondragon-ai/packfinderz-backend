package models

import (
	"time"

	"github.com/google/uuid"
)

// InventoryItem tracks available/reserved counts per product.
type InventoryItem struct {
	ProductID    uuid.UUID `gorm:"column:product_id;type:uuid;primaryKey"`
	AvailableQty int       `gorm:"column:available_qty;not null;default:0"`
	ReservedQty  int       `gorm:"column:reserved_qty;not null;default:0"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
}
