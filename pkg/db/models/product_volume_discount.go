package models

import (
	"time"

	"github.com/google/uuid"
)

// ProductVolumeDiscount captures tiered pricing per product.
type ProductVolumeDiscount struct {
	ID             uuid.UUID `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	StoreID        uuid.UUID `gorm:"column:store_id;type:uuid;not null"`
	ProductID      uuid.UUID `gorm:"column:product_id;type:uuid;not null"`
	MinQty         int       `gorm:"column:min_qty;not null"`
	UnitPriceCents int       `gorm:"column:unit_price_cents;not null"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
}
