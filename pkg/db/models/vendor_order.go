package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// VendorOrder represents the per-vendor order produced from a checkout group.
type VendorOrder struct {
	ID                uuid.UUID                          `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	CheckoutGroupID   uuid.UUID                          `gorm:"column:checkout_group_id;type:uuid;not null"`
	BuyerStoreID      uuid.UUID                          `gorm:"column:buyer_store_id;type:uuid;not null"`
	VendorStoreID     uuid.UUID                          `gorm:"column:vendor_store_id;type:uuid;not null"`
	Status            enums.VendorOrderStatus            `gorm:"column:status;type:vendor_order_status;not null;default:'created_pending'"`
	RefundStatus      enums.RefundStatus                 `gorm:"column:refund_status;type:refund_status;not null;default:'none'"`
	SubtotalCents     int                                `gorm:"column:subtotal_cents;not null"`
	DiscountCents     int                                `gorm:"column:discount_cents;not null;default:0"`
	TaxCents          int                                `gorm:"column:tax_cents;not null;default:0"`
	TransportFeeCents int                                `gorm:"column:transport_fee_cents;not null;default:0"`
	TotalCents        int                                `gorm:"column:total_cents;not null"`
	BalanceDueCents   int                                `gorm:"column:balance_due_cents;not null;default:0"`
	FulfillmentStatus enums.VendorOrderFulfillmentStatus `gorm:"column:fulfillment_status;type:vendor_order_fulfillment_status;not null;default:'pending'"`
	ShippingStatus    enums.VendorOrderShippingStatus    `gorm:"column:shipping_status;type:vendor_order_shipping_status;not null;default:'pending'"`
	OrderNumber       int64                              `gorm:"column:order_number;not null"`
	Notes             *string                            `gorm:"column:notes"`
	InternalNotes     *string                            `gorm:"column:internal_notes"`
	FulfilledAt       *time.Time                         `gorm:"column:fulfilled_at"`
	DeliveredAt       *time.Time                         `gorm:"column:delivered_at"`
	CanceledAt        *time.Time                         `gorm:"column:canceled_at"`
	ExpiredAt         *time.Time                         `gorm:"column:expired_at"`
	Items             []OrderLineItem                    `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	PaymentIntent     *PaymentIntent                     `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	Assignments       []OrderAssignment                  `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	CreatedAt         time.Time                          `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time                          `gorm:"column:updated_at;autoUpdateTime"`
}
