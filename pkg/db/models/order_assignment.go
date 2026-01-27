package models

import (
	"time"

	"github.com/google/uuid"
)

// OrderAssignment captures agent assignment history for a vendor order.
type OrderAssignment struct {
	ID                      uuid.UUID  `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	OrderID                 uuid.UUID  `gorm:"column:order_id;type:uuid;not null"`
	AgentUserID             uuid.UUID  `gorm:"column:agent_user_id;type:uuid;not null"`
	AssignedByUserID        *uuid.UUID `gorm:"column:assigned_by_user_id;type:uuid"`
	AssignedAt              time.Time  `gorm:"column:assigned_at;autoCreateTime"`
	UnassignedAt            *time.Time `gorm:"column:unassigned_at"`
	Active                  bool       `gorm:"column:active;not null;default:true"`
	PickupTime              *time.Time `gorm:"column:pickup_time"`
	DeliveryTime            *time.Time `gorm:"column:delivery_time"`
	CashPickupTime          *time.Time `gorm:"column:cash_pickup_time"`
	PickupSignatureGCSKey   *string    `gorm:"column:pickup_signature_gcs_key"`
	DeliverySignatureGCSKey *string    `gorm:"column:delivery_signature_gcs_key"`
}
