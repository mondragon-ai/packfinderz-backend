package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// CartItem persists product-level snapshots tied to a CartRecord.
type CartItem struct {
	ID                              uuid.UUID         `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	CartID                          uuid.UUID         `gorm:"column:cart_id;type:uuid;not null"`
	ProductID                       uuid.UUID         `gorm:"column:product_id;type:uuid;not null"`
	VendorStoreID                   uuid.UUID         `gorm:"column:vendor_store_id;type:uuid;not null"`
	Qty                             int               `gorm:"column:qty;not null"`
	ProductSKU                      string            `gorm:"column:product_sku;not null"`
	Unit                            enums.ProductUnit `gorm:"column:unit;type:unit;not null"`
	UnitPriceCents                  int               `gorm:"column:unit_price_cents;not null"`
	CompareAtUnitPriceCents         *int              `gorm:"column:compare_at_unit_price_cents"`
	AppliedVolumeTierMinQty         *int              `gorm:"column:applied_volume_tier_min_qty"`
	AppliedVolumeTierUnitPriceCents *int              `gorm:"column:applied_volume_tier_unit_price_cents"`
	DiscountedPrice                 *int              `gorm:"column:discounted_price"`
	SubTotalPrice                   *int              `gorm:"column:sub_total_price"`
	FeaturedImage                   *string           `gorm:"column:featured_image"`
	MOQ                             *int              `gorm:"column:moq"`
	THCPercent                      *float64          `gorm:"column:thc_percent;type:numeric(5,2)"`
	CBDPercent                      *float64          `gorm:"column:cbd_percent;type:numeric(5,2)"`
	CreatedAt                       time.Time         `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt                       time.Time         `gorm:"column:updated_at;autoUpdateTime"`
}
