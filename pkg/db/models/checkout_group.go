package models

import (
	"time"

	"github.com/google/uuid"
)

// CheckoutGroup links a buyer store's checkout attempt to the resulting vendor orders.
type CheckoutGroup struct {
	ID                  uuid.UUID         `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	BuyerStoreID        uuid.UUID         `gorm:"column:buyer_store_id;type:uuid;not null"`
	CartID              *uuid.UUID        `gorm:"column:cart_id;type:uuid"`
	AttributedAdClickID *uuid.UUID        `gorm:"column:attributed_ad_click_id;type:uuid"`
	VendorOrders        []VendorOrder     `gorm:"foreignKey:CheckoutGroupID;constraint:OnDelete:CASCADE"`
	CreatedAt           time.Time         `gorm:"column:created_at;autoCreateTime"`
	CartVendorGroups    []CartVendorGroup `gorm:"-"`
}
