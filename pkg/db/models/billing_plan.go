package models

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// BillingPlan captures the local metadata for a subscription plan.
type BillingPlan struct {
	ID                        string                `gorm:"column:id;primaryKey"`
	Name                      string                `gorm:"column:name;not null"`
	Status                    enums.PlanStatus      `gorm:"column:status;type:plan_status;not null"`
	SquareBillingPlanID       string                `gorm:"column:square_billing_plan_id;not null;uniqueIndex"`
	Test                      bool                  `gorm:"column:test;not null;default:false"`
	IsDefault                 bool                  `gorm:"column:is_default;not null;default:false"`
	TrialDays                 int                   `gorm:"column:trial_days;not null;default:0"`
	TrialRequirePaymentMethod bool                  `gorm:"column:trial_require_payment_method;not null;default:false"`
	TrialStartOnActivation    bool                  `gorm:"column:trial_start_on_activation;not null;default:true"`
	Interval                  enums.BillingInterval `gorm:"column:interval;type:billing_interval;not null"`
	PriceAmount               decimal.Decimal       `gorm:"column:price_amount;type:numeric(12,2);not null"`
	CurrencyCode              string                `gorm:"column:currency_code;not null"`
	Features                  pq.StringArray        `gorm:"column:features;type:text[];default:ARRAY[]::text[]"`
	UI                        json.RawMessage       `gorm:"column:ui;type:jsonb"`
	CreatedAt                 time.Time             `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt                 time.Time             `gorm:"column:updated_at;autoUpdateTime"`
}
