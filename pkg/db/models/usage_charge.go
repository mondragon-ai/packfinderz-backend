package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// UsageCharge captures Stripe usage charges per store.
type UsageCharge struct {
	ID                  uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	StoreID             uuid.UUID       `gorm:"column:store_id;type:uuid;not null;index"`
	SubscriptionID      *uuid.UUID      `gorm:"column:subscription_id;type:uuid"`
	ChargeID            *uuid.UUID      `gorm:"column:charge_id;type:uuid"`
	StripeUsageChargeID string          `gorm:"column:stripe_usage_charge_id;not null;unique"`
	Quantity            int64           `gorm:"column:quantity;not null"`
	AmountCents         int64           `gorm:"column:amount_cents;not null"`
	Currency            string          `gorm:"column:currency;not null;default:'usd'"`
	Description         *string         `gorm:"column:description"`
	BillingPeriodStart  *time.Time      `gorm:"column:billing_period_start"`
	BillingPeriodEnd    *time.Time      `gorm:"column:billing_period_end"`
	Metadata            json.RawMessage `gorm:"column:metadata;type:jsonb"`
	CreatedAt           time.Time       `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time       `gorm:"column:updated_at;autoUpdateTime"`
}
