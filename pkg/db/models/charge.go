package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// Charge records Stripe charges per store.
type Charge struct {
	ID              uuid.UUID          `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	StoreID         uuid.UUID          `gorm:"column:store_id;type:uuid;not null;index"`
	Type            enums.ChargeType   `gorm:"column:type;type:charge_type;not null;default:'subscription'"`
	SubscriptionID  *uuid.UUID         `gorm:"column:subscription_id;type:uuid"`
	PaymentMethodID *uuid.UUID         `gorm:"column:payment_method_id;type:uuid"`
	StripeChargeID  string             `gorm:"column:stripe_charge_id;not null;unique"`
	AmountCents     int64              `gorm:"column:amount_cents;not null"`
	Currency        string             `gorm:"column:currency;not null;default:'usd'"`
	Status          enums.ChargeStatus `gorm:"column:status;type:charge_status;not null;default:'pending'"`
	Description     *string            `gorm:"column:description"`
	BilledAt        *time.Time         `gorm:"column:billed_at"`
	Metadata        json.RawMessage    `gorm:"column:metadata;type:jsonb"`
	CreatedAt       time.Time          `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time          `gorm:"column:updated_at;autoUpdateTime"`
}
