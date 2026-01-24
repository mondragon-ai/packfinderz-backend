package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

// CartRecord captures a buyer-scoped cart snapshot persisted at checkout confirmation.
type CartRecord struct {
	ID                 uuid.UUID                `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	BuyerStoreID       uuid.UUID                `gorm:"column:buyer_store_id;type:uuid;not null"`
	SessionID          *string                  `gorm:"column:session_id"`
	Status             enums.CartStatus         `gorm:"column:status;type:cart_status;not null;default:'active'"`
	ShippingAddress    *types.Address           `gorm:"column:shipping_address;type:address_t"`
	TotalDiscount      int                      `gorm:"column:total_discount;not null;default:0"`
	Fees               int                      `gorm:"column:fees;not null;default:0"`
	SubtotalCents      int                      `gorm:"column:subtotal_cents;not null;default:0"`
	TotalCents         int                      `gorm:"column:total_cents;not null;default:0"`
	CartLevelDiscounts types.CartLevelDiscounts `gorm:"column:cart_level_discount;type:cart_level_discount[]"`
	Items              []CartItem               `gorm:"foreignKey:CartID;constraint:OnDelete:CASCADE"`
	CreatedAt          time.Time                `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time                `gorm:"column:updated_at;autoUpdateTime"`
}
