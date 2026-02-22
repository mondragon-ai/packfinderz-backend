package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

// CartItem persists product-level snapshots tied to a CartRecord.
type CartItem struct {
	ID                      uuid.UUID                    `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	CartID                  uuid.UUID                    `gorm:"column:cart_id;type:uuid;not null"`
	ProductID               uuid.UUID                    `gorm:"column:product_id;type:uuid;not null"`
	VendorStoreID           uuid.UUID                    `gorm:"column:vendor_store_id;type:uuid;not null"`
	VendorStoreName         string                       `gorm:"column:vendor_store_name;not null"`
	Unit                    enums.ProductUnit            `gorm:"column:unit;not null"`
	Quantity                int                          `gorm:"column:quantity;not null"`
	Title                   string                       `gorm:"column:title;not null"`
	Thumbnail               *string                      `gorm:"column:thumbnail"`
	MOQ                     int                          `gorm:"column:moq;not null;default:1"`
	MaxQty                  *int                         `gorm:"column:max_qty"`
	UnitPriceCents          int                          `gorm:"column:unit_price_cents;not null"`
	EffectiveUnitPriceCents int                          `gorm:"column:effective_unit_price_cents;not null;default:0"`
	LineDiscountsCents      int                          `gorm:"column:line_discounts_cents;not null;default:0"`
	LineTotalCents          int                          `gorm:"column:line_total_cents;not null;default:0"`
	AppliedVolumeDiscount   *types.AppliedVolumeDiscount `gorm:"column:applied_volume_discount;type:jsonb;serializer:json"`
	Warnings                types.CartItemWarnings       `gorm:"column:warnings;type:jsonb;serializer:json"`
	LineSubtotalCents       int                          `gorm:"column:line_subtotal_cents;not null"`
	Status                  enums.CartItemStatus         `gorm:"column:status;type:cart_item_status;not null;default:'ok'"`
	CreatedAt               time.Time                    `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt               time.Time                    `gorm:"column:updated_at;autoUpdateTime"`
}
