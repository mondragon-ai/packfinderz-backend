package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

// CartVendorGroup persists vendor-level aggregates for the authoritative cart quote.
type CartVendorGroup struct {
	ID            uuid.UUID                 `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	CartID        uuid.UUID                 `gorm:"column:cart_id;type:uuid;not null"`
	VendorStoreID uuid.UUID                 `gorm:"column:vendor_store_id;type:uuid;not null"`
	Status        enums.VendorGroupStatus   `gorm:"column:status;type:vendor_group_status;not null;default:'ok'"`
	Warnings      types.VendorGroupWarnings `gorm:"column:warnings;type:jsonb"`
	SubtotalCents int                       `gorm:"column:subtotal_cents;not null;default:0"`
	Promo         *types.VendorGroupPromo   `gorm:"column:promo;type:jsonb"`
	TotalCents    int                       `gorm:"column:total_cents;not null;default:0"`
	CreatedAt     time.Time                 `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time                 `gorm:"column:updated_at;autoUpdateTime"`
}
