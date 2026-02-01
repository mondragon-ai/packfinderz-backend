package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

// OrderLineItem captures the snapshot of each item within a vendor order.
type OrderLineItem struct {
	ID                    uuid.UUID                    `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	OrderID               uuid.UUID                    `gorm:"column:order_id;type:uuid;not null"`
	ProductID             *uuid.UUID                   `gorm:"column:product_id;type:uuid"`
	CartItemID            *uuid.UUID                   `gorm:"column:cart_item_id;type:uuid"`
	Name                  string                       `gorm:"column:name;not null"`
	Category              string                       `gorm:"column:category;not null"`
	Strain                *string                      `gorm:"column:strain"`
	Classification        *string                      `gorm:"column:classification"`
	Unit                  enums.ProductUnit            `gorm:"column:unit;type:unit;not null"`
	MOQ                   int                          `gorm:"column:moq;not null"`
	MaxQty                *int                         `gorm:"column:max_qty"`
	UnitPriceCents        int                          `gorm:"column:unit_price_cents;not null"`
	Qty                   int                          `gorm:"column:qty;not null"`
	DiscountCents         int                          `gorm:"column:discount_cents;not null;default:0"`
	LineSubtotalCents     int                          `gorm:"column:line_subtotal_cents;not null"`
	TotalCents            int                          `gorm:"column:total_cents;not null"`
	Warnings              types.CartItemWarnings       `gorm:"column:warnings;type:jsonb;serializer:json"`
	AppliedVolumeDiscount *types.AppliedVolumeDiscount `gorm:"column:applied_volume_discount;type:jsonb;serializer:json"`
	AttributedToken       *types.JSONMap               `gorm:"column:attributed_token;type:jsonb;serializer:json"`
	Status                enums.LineItemStatus         `gorm:"column:status;type:line_item_status;not null;default:'pending'"`
	Notes                 *string                      `gorm:"column:notes"`
	CreatedAt             time.Time                    `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt             time.Time                    `gorm:"column:updated_at;autoUpdateTime"`
}
