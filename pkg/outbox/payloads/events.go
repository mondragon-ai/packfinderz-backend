package payloads

import (
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
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

// OrderReadyForDispatchEvent mirrors the payload emitted once all line items resolve.
type OrderReadyForDispatchEvent struct {
	OrderID            uuid.UUID                          `json:"order_id"`
	CheckoutGroupID    uuid.UUID                          `json:"checkout_group_id"`
	BuyerStoreID       uuid.UUID                          `json:"buyer_store_id"`
	VendorStoreID      uuid.UUID                          `json:"vendor_store_id"`
	VendorStoreIDs     []uuid.UUID                        `json:"vendor_store_ids"`
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

// CashCollectedEvent captures the payload emitted once an agent collects cash.
type CashCollectedEvent struct {
	OrderID         uuid.UUID `json:"order_id"`
	BuyerStoreID    uuid.UUID `json:"buyer_store_id"`
	VendorStoreID   uuid.UUID `json:"vendor_store_id"`
	AmountCents     int       `json:"amount_cents"`
	CashCollectedAt time.Time `json:"cash_collected_at"`
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

// CheckoutConvertedEvent informs notifications consumers that a checkout finished.
type CheckoutConvertedEvent struct {
	CheckoutGroupID uuid.UUID                       `json:"checkout_group_id"`
	CartID          *uuid.UUID                      `json:"cart_id,omitempty"`
	BuyerStoreID    uuid.UUID                       `json:"buyer_store_id"`
	VendorOrderIDs  []uuid.UUID                     `json:"vendor_order_ids"`
	VendorStoreIDs  []uuid.UUID                     `json:"vendor_store_ids"`
	ConvertedAt     time.Time                       `json:"converted_at"`
	Analytics       CheckoutConvertedAnalyticsEvent `json:"analytics"`
}

// ShippingAddress mirrors the canonical subset used by analytics payloads.
type ShippingAddress struct {
	PostalCode string  `json:"postal_code"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
}

// CheckoutConvertedAnalyticsVendor summarizes a vendor order for analytics.
type CheckoutConvertedAnalyticsVendor struct {
	OrderID             string `json:"order_id"`
	VendorStoreID       string `json:"vendor_store_id"`
	Status              string `json:"status"`
	PaymentStatus       string `json:"payment_status"`
	PaymentMethod       string `json:"payment_method"`
	TotalCents          int64  `json:"total_cents"`
	BalanceDueCents     int64  `json:"balance_due_cents"`
	PaymentIntentStatus string `json:"payment_intent_status"`
	PaymentIntentMethod string `json:"payment_intent_method"`
}

// CheckoutConvertedAnalyticsItem captures a line item snapshot for analytics.
type CheckoutConvertedAnalyticsItem struct {
	OrderID               string                       `json:"order_id"`
	VendorStoreID         string                       `json:"vendor_store_id"`
	ProductID             string                       `json:"product_id"`
	Name                  string                       `json:"name"`
	Category              string                       `json:"category"`
	Strain                *string                      `json:"strain,omitempty"`
	Classification        *string                      `json:"classification,omitempty"`
	Unit                  string                       `json:"unit"`
	MOQ                   int                          `json:"moq"`
	MaxQty                *int                         `json:"max_qty,omitempty"`
	Qty                   int                          `json:"qty"`
	UnitPriceCents        int                          `json:"unit_price_cents"`
	DiscountCents         int                          `json:"discount_cents"`
	LineSubtotalCents     int                          `json:"line_subtotal_cents"`
	LineTotalCents        int                          `json:"line_total_cents"`
	Status                string                       `json:"status"`
	Warnings              []types.CartItemWarning      `json:"warnings,omitempty"`
	AppliedVolumeDiscount *types.AppliedVolumeDiscount `json:"applied_volume_discount,omitempty"`
	AttributedToken       *types.JSONMap               `json:"attributed_token,omitempty"`
}

// CheckoutConvertedAnalyticsEvent contains the cart snapshot emitted when checkout converts.
type CheckoutConvertedAnalyticsEvent struct {
	Currency        string                             `json:"currency"`
	PaymentMethod   string                             `json:"payment_method"`
	ShippingAddress *ShippingAddress                   `json:"shipping_address,omitempty"`
	ShippingLine    *types.ShippingLine                `json:"shipping_line,omitempty"`
	SubtotalCents   int64                              `json:"subtotal_cents"`
	DiscountsCents  int64                              `json:"discounts_cents"`
	TotalCents      int64                              `json:"total_cents"`
	VendorOrders    []CheckoutConvertedAnalyticsVendor `json:"vendor_orders"`
	Items           []CheckoutConvertedAnalyticsItem   `json:"items"`
	AdTokens        []string                           `json:"ad_tokens,omitempty"`
}

// LicenseStatusChangedEvent mirrors the payload emitted when license status updates.
type LicenseStatusChangedEvent struct {
	LicenseID   uuid.UUID           `json:"licenseId"`
	StoreID     uuid.UUID           `json:"storeId"`
	Status      enums.LicenseStatus `json:"status"`
	Reason      string              `json:"reason,omitempty"`
	WarningType string              `json:"warningType,omitempty"`
}
