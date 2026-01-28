package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// Subscription persists Stripe subscription state per store.
type Subscription struct {
	ID                   uuid.UUID                `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	StoreID              uuid.UUID                `gorm:"column:store_id;type:uuid;not null;index"`
	StripeSubscriptionID string                   `gorm:"column:stripe_subscription_id;not null;unique"`
	Status               enums.SubscriptionStatus `gorm:"column:status;type:subscription_status;not null;default:'active'"`
	PriceID              *string                  `gorm:"column:price_id"`
	CurrentPeriodStart   *time.Time               `gorm:"column:current_period_start"`
	CurrentPeriodEnd     time.Time                `gorm:"column:current_period_end;not null"`
	CancelAtPeriodEnd    bool                     `gorm:"column:cancel_at_period_end;not null;default:false"`
	CanceledAt           *time.Time               `gorm:"column:canceled_at"`
	Metadata             json.RawMessage          `gorm:"column:metadata;type:jsonb"`
	CreatedAt            time.Time                `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt            time.Time                `gorm:"column:updated_at;autoUpdateTime"`
}
