package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

// CartRecord captures a buyer-scoped cart snapshot persisted at checkout confirmation.
type CartRecord struct {
	ID              uuid.UUID            `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	BuyerStoreID    uuid.UUID            `gorm:"column:buyer_store_id;type:uuid;not null"`
	CheckoutGroupID *uuid.UUID           `gorm:"column:checkout_group_id;type:uuid"`
	Status          enums.CartStatus     `gorm:"column:status;type:cart_status;not null;default:'active'"`
	ShippingAddress *types.Address       `gorm:"column:shipping_address;type:address_t"`
	PaymentMethod   *enums.PaymentMethod `gorm:"column:payment_method;type:payment_method"`
	ShippingLine    *types.ShippingLine  `gorm:"column:shipping_line;type:jsonb;serializer:json"`
	Currency        enums.Currency       `gorm:"column:currency;not null;default:'USD'"`
	ValidUntil      time.Time            `gorm:"column:valid_until;not null"`
	SubtotalCents   int                  `gorm:"column:subtotal_cents;not null;default:0"`
	DiscountsCents  int                  `gorm:"column:discounts_cents;not null;default:0"`
	TotalCents      int                  `gorm:"column:total_cents;not null;default:0"`
	ConvertedAt     *time.Time           `gorm:"column:converted_at"`
	AdTokens        pq.StringArray       `gorm:"column:ad_tokens;type:text[]"`
	VendorGroups    []CartVendorGroup    `gorm:"foreignKey:CartID;constraint:OnDelete:CASCADE"`
	Items           []CartItem           `gorm:"foreignKey:CartID;constraint:OnDelete:CASCADE"`
	CreatedAt       time.Time            `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time            `gorm:"column:updated_at;autoUpdateTime"`
}
