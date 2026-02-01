package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

// VendorOrder represents the per-vendor order produced from a checkout group.
type VendorOrder struct {
	ID                uuid.UUID                          `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	CartID            uuid.UUID                          `gorm:"column:cart_id;type:uuid;not null"`
	CheckoutGroupID   uuid.UUID                          `gorm:"column:checkout_group_id;type:uuid;not null"`
	BuyerStoreID      uuid.UUID                          `gorm:"column:buyer_store_id;type:uuid;not null"`
	VendorStoreID     uuid.UUID                          `gorm:"column:vendor_store_id;type:uuid;not null"`
	Currency          enums.Currency                     `gorm:"column:currency;type:text;not null;default:'USD'"`
	ShippingAddress   *types.Address                     `gorm:"column:shipping_address;type:address_t"`
	Status            enums.VendorOrderStatus            `gorm:"column:status;type:vendor_order_status;not null;default:'created_pending'"`
	RefundStatus      enums.RefundStatus                 `gorm:"column:refund_status;type:refund_status;not null;default:'none'"`
	SubtotalCents     int                                `gorm:"column:subtotal_cents;not null"`
	DiscountsCents    int                                `gorm:"column:discounts_cents;not null;default:0"`
	TaxCents          int                                `gorm:"column:tax_cents;not null;default:0"`
	TransportFeeCents int                                `gorm:"column:transport_fee_cents;not null;default:0"`
	PaymentMethod     enums.PaymentMethod                `gorm:"column:payment_method;type:payment_method;not null;default:'cash'"`
	TotalCents        int                                `gorm:"column:total_cents;not null"`
	BalanceDueCents   int                                `gorm:"column:balance_due_cents;not null;default:0"`
	FulfillmentStatus enums.VendorOrderFulfillmentStatus `gorm:"column:fulfillment_status;type:vendor_order_fulfillment_status;not null;default:'pending'"`
	ShippingStatus    enums.VendorOrderShippingStatus    `gorm:"column:shipping_status;type:vendor_order_shipping_status;not null;default:'pending'"`
	OrderNumber       int64                              `gorm:"column:order_number;not null"`
	Notes             *string                            `gorm:"column:notes"`
	InternalNotes     *string                            `gorm:"column:internal_notes"`
	Warnings          types.VendorGroupWarnings          `gorm:"column:warnings;type:jsonb;serializer:json"`
	Promo             *types.VendorGroupPromo            `gorm:"column:promo;type:jsonb;serializer:json"`
	ShippingLine      *types.ShippingLine                `gorm:"column:shipping_line;type:jsonb;serializer:json"`
	AttributedToken   *types.JSONMap                     `gorm:"column:attributed_token;type:jsonb;serializer:json"`
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
