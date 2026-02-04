package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// PaymentIntent tracks payment progress for a vendor order.
type PaymentIntent struct {
	ID              uuid.UUID           `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	OrderID         uuid.UUID           `gorm:"column:order_id;type:uuid;not null"`
	Method          enums.PaymentMethod `gorm:"column:method;type:payment_method;not null;default:'cash'"`
	Status          enums.PaymentStatus `gorm:"column:status;type:payment_status;not null;default:'unpaid'"`
	AmountCents     int                 `gorm:"column:amount_cents;not null"`
	CashCollectedAt *time.Time          `gorm:"column:cash_collected_at"`
	VendorPaidAt    *time.Time          `gorm:"column:vendor_paid_at"`
	FailureReason   *string             `gorm:"column:failure_reason"`
	CreatedAt       time.Time           `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time           `gorm:"column:updated_at;autoUpdateTime"`
}
