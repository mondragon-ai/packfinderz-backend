package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// LedgerEvent records an immutable money lifecycle event tied to a vendor order.
type LedgerEvent struct {
	ID            uuid.UUID             `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	OrderID       uuid.UUID             `gorm:"column:order_id;type:uuid;not null"`
	BuyerStoreID  uuid.UUID             `gorm:"column:buyer_store_id;type:uuid;not null"`
	VendorStoreID uuid.UUID             `gorm:"column:vendor_store_id;type:uuid;not null"`
	ActorUserID   uuid.UUID             `gorm:"column:actor_user_id;type:uuid;not null"`
	Type          enums.LedgerEventType `gorm:"column:type;type:ledger_event_type_enum;not null"`
	AmountCents   int                   `gorm:"column:amount_cents;not null"`
	Metadata      json.RawMessage       `gorm:"column:metadata;type:jsonb"`
	CreatedAt     time.Time             `gorm:"column:created_at;autoCreateTime"`
}
