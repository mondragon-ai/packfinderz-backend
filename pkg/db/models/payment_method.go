package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// PaymentMethod mirrors Stripe payment methods per store.
type PaymentMethod struct {
	ID                    uuid.UUID               `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	StoreID               uuid.UUID               `gorm:"column:store_id;type:uuid;not null;index"`
	StripePaymentMethodID string                  `gorm:"column:stripe_payment_method_id;not null;unique"`
	Type                  enums.PaymentMethodType `gorm:"column:type;type:payment_method_type;not null;default:'card'"`
	Fingerprint           *string                 `gorm:"column:fingerprint"`
	CardBrand             *string                 `gorm:"column:card_brand"`
	CardLast4             *string                 `gorm:"column:card_last4"`
	CardExpMonth          *int                    `gorm:"column:card_exp_month"`
	CardExpYear           *int                    `gorm:"column:card_exp_year"`
	BillingDetails        json.RawMessage         `gorm:"column:billing_details;type:jsonb"`
	Metadata              json.RawMessage         `gorm:"column:metadata;type:jsonb"`
	CreatedAt             time.Time               `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt             time.Time               `gorm:"column:updated_at;autoUpdateTime"`
}
