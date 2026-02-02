package payloads

import (
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

// OrderCreatedEvent signals a new checkout split across vendors.
type OrderCreatedEvent struct {
	CheckoutGroupID uuid.UUID   `json:"checkout_group_id"`
	VendorOrderIDs  []uuid.UUID `json:"vendor_order_ids"`
}

// OrderDecisionEvent is emitted when a vendor decides an order.
type OrderDecisionEvent struct {
	OrderID         uuid.UUID                 `json:"order_id"`
	CheckoutGroupID uuid.UUID                 `json:"checkout_group_id"`
	BuyerStoreID    uuid.UUID                 `json:"buyer_store_id"`
	VendorStoreID   uuid.UUID                 `json:"vendor_store_id"`
	Decision        enums.VendorOrderDecision `json:"decision"`
	Status          enums.VendorOrderStatus   `json:"status"`
}

// OrderFulfilledEvent surfaces the aggregated fields when fulfillment completes.
type OrderFulfilledEvent struct {
	OrderID            uuid.UUID                          `json:"order_id"`
	CheckoutGroupID    uuid.UUID                          `json:"checkout_group_id"`
	BuyerStoreID       uuid.UUID                          `json:"buyer_store_id"`
	VendorStoreID      uuid.UUID                          `json:"vendor_store_id"`
	FulfillmentStatus  enums.VendorOrderFulfillmentStatus `json:"fulfillment_status"`
	ShippingStatus     enums.VendorOrderShippingStatus    `json:"shipping_status"`
	RejectedItemCount  int                                `json:"rejected_item_count"`
	ResolvedLineItemID uuid.UUID                          `json:"resolved_line_item_id"`
}

// OrderCanceledEvent is emitted whenever a buyer cancels a pre-transit order.
type OrderCanceledEvent struct {
	OrderID         uuid.UUID `json:"order_id"`
	CheckoutGroupID uuid.UUID `json:"checkout_group_id"`
	BuyerStoreID    uuid.UUID `json:"buyer_store_id"`
	VendorStoreID   uuid.UUID `json:"vendor_store_id"`
	CanceledAt      time.Time `json:"canceled_at"`
	Reason          string    `json:"reason,omitempty"`
}

// NotificationRequestedEvent tells downstream systems to alert a vendor.
type NotificationRequestedEvent struct {
	OrderID         uuid.UUID `json:"order_id"`
	CheckoutGroupID uuid.UUID `json:"checkout_group_id"`
	BuyerStoreID    uuid.UUID `json:"buyer_store_id"`
	VendorStoreID   uuid.UUID `json:"vendor_store_id"`
	Type            string    `json:"type"`
}

// OrderRetriedEvent reports that an expired order was replayed.
type OrderRetriedEvent struct {
	OriginalOrderID uuid.UUID `json:"original_order_id"`
	OrderID         uuid.UUID `json:"order_id"`
	CheckoutGroupID uuid.UUID `json:"checkout_group_id"`
	BuyerStoreID    uuid.UUID `json:"buyer_store_id"`
	VendorStoreID   uuid.UUID `json:"vendor_store_id"`
}

// OrderPaidEvent is emitted when admin confirms payout and the vendor has been paid.
type OrderPaidEvent struct {
	OrderID         uuid.UUID `json:"order_id"`
	BuyerStoreID    uuid.UUID `json:"buyer_store_id"`
	VendorStoreID   uuid.UUID `json:"vendor_store_id"`
	PaymentIntentID uuid.UUID `json:"payment_intent_id"`
	AmountCents     int       `json:"amount_cents"`
	VendorPaidAt    time.Time `json:"vendor_paid_at"`
}

// OrderPendingNudgeEvent carries the payload for nudges.
type OrderPendingNudgeEvent struct {
	OrderID         uuid.UUID `json:"orderId"`
	CheckoutGroupID uuid.UUID `json:"checkoutGroupId"`
	BuyerStoreID    uuid.UUID `json:"buyerStoreId"`
	VendorStoreID   uuid.UUID `json:"vendorStoreId"`
	PendingDays     int       `json:"pendingDays"`
}

// OrderExpiredEvent describes the payload when orders expire.
type OrderExpiredEvent struct {
	OrderID         uuid.UUID `json:"orderId"`
	CheckoutGroupID uuid.UUID `json:"checkoutGroupId"`
	BuyerStoreID    uuid.UUID `json:"buyerStoreId"`
	VendorStoreID   uuid.UUID `json:"vendorStoreId"`
	ExpiredAt       time.Time `json:"expiredAt"`
	TTLDays         *int      `json:"ttl_days,omitempty"`
}

// LicenseExpiringSoonEvent describes the payload for the warning.
type LicenseExpiringSoonEvent struct {
	LicenseID           uuid.UUID `json:"licenseId"`
	StoreID             uuid.UUID `json:"storeId"`
	ExpirationDate      time.Time `json:"expirationDate"`
	DaysUntilExpiration int       `json:"daysUntilExpiration"`
}

// LicenseExpiredEvent describes the payload for expired licenses.
type LicenseExpiredEvent struct {
	LicenseID      uuid.UUID `json:"licenseId"`
	StoreID        uuid.UUID `json:"storeId"`
	ExpirationDate time.Time `json:"expirationDate"`
	ExpiredAt      time.Time `json:"expiredAt"`
}

// LicenseStatusChangedEvent mirrors the payload emitted when license status updates.
type LicenseStatusChangedEvent struct {
	LicenseID   uuid.UUID           `json:"licenseId"`
	StoreID     uuid.UUID           `json:"storeId"`
	Status      enums.LicenseStatus `json:"status"`
	Reason      string              `json:"reason,omitempty"`
	WarningType string              `json:"warningType,omitempty"`
}
