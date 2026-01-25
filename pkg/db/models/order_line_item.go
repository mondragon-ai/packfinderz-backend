package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// OrderLineItem captures the snapshot of each item within a vendor order.
type OrderLineItem struct {
	ID             uuid.UUID            `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	OrderID        uuid.UUID            `gorm:"column:order_id;type:uuid;not null"`
	ProductID      *uuid.UUID           `gorm:"column:product_id;type:uuid"`
	Name           string               `gorm:"column:name;not null"`
	Category       string               `gorm:"column:category;not null"`
	Strain         *string              `gorm:"column:strain"`
	Classification *string              `gorm:"column:classification"`
	Unit           enums.ProductUnit    `gorm:"column:unit;type:unit;not null"`
	UnitPriceCents int                  `gorm:"column:unit_price_cents;not null"`
	Qty            int                  `gorm:"column:qty;not null"`
	DiscountCents  int                  `gorm:"column:discount_cents;not null;default:0"`
	TotalCents     int                  `gorm:"column:total_cents;not null"`
	Status         enums.LineItemStatus `gorm:"column:status;type:line_item_status;not null;default:'pending'"`
	Notes          *string              `gorm:"column:notes"`
	CreatedAt      time.Time            `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time            `gorm:"column:updated_at;autoUpdateTime"`
}
