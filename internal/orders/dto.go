package orders

import (
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

// BuyerOrderFilters describe the inputs supported by the buyer orders list.
type BuyerOrderFilters struct {
	OrderStatus       *enums.VendorOrderStatus
	FulfillmentStatus *enums.VendorOrderFulfillmentStatus
	ShippingStatus    *enums.VendorOrderShippingStatus
	PaymentStatus     *enums.PaymentStatus
	DateFrom          *time.Time
	DateTo            *time.Time
	Query             string
}

// VendorOrderFilters describe the inputs supported by the vendor orders list.
type VendorOrderFilters struct {
	OrderStatus        *enums.VendorOrderStatus
	FulfillmentStatus  *enums.VendorOrderFulfillmentStatus
	ShippingStatus     *enums.VendorOrderShippingStatus
	PaymentStatus      *enums.PaymentStatus
	DateFrom           *time.Time
	DateTo             *time.Time
	ActionableStatuses []enums.VendorOrderStatus
	Query              string
}

// OrderStoreSummary captures the vendor summary returned in the order list.
type OrderStoreSummary struct {
	ID          uuid.UUID `json:"id"`
	CompanyName string    `json:"company_name"`
	DBAName     *string   `json:"dba_name,omitempty"`
	LogoURL     *string   `json:"logo_url,omitempty"`
}

// BuyerOrderSummary exposes the aggregated fields returned in the buyer list.
type BuyerOrderSummary struct {
	OrderNumber       int64                              `json:"order_number"`
	CreatedAt         time.Time                          `json:"created_at"`
	TotalCents        int                                `json:"total_cents"`
	DiscountCents     int                                `json:"discount_cents"`
	TotalItems        int                                `json:"total_items"`
	PaymentStatus     enums.PaymentStatus                `json:"payment_status"`
	FulfillmentStatus enums.VendorOrderFulfillmentStatus `json:"fulfillment_status"`
	ShippingStatus    enums.VendorOrderShippingStatus    `json:"shipping_status"`
	Vendor            OrderStoreSummary                  `json:"vendor"`
}

// BuyerOrderList wraps the paginated orders plus the next page cursor.
type BuyerOrderList struct {
	Orders     []BuyerOrderSummary `json:"orders"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

// VendorOrderSummary exposes aggregated fields returned in the vendor list.
type VendorOrderSummary struct {
	OrderNumber       int64                              `json:"order_number"`
	CreatedAt         time.Time                          `json:"created_at"`
	TotalCents        int                                `json:"total_cents"`
	DiscountCents     int                                `json:"discount_cents"`
	TotalItems        int                                `json:"total_items"`
	PaymentStatus     enums.PaymentStatus                `json:"payment_status"`
	FulfillmentStatus enums.VendorOrderFulfillmentStatus `json:"fulfillment_status"`
	ShippingStatus    enums.VendorOrderShippingStatus    `json:"shipping_status"`
	Buyer             OrderStoreSummary                  `json:"buyer"`
}

// VendorOrderList wraps paginated vendor orders plus the next cursor.
type VendorOrderList struct {
	Orders     []VendorOrderSummary `json:"orders"`
	NextCursor string               `json:"next_cursor,omitempty"`
}

// OrderAssignmentSummary highlights the active agent assignment for an order.
type OrderAssignmentSummary struct {
	AgentUserID      uuid.UUID  `json:"agent_user_id"`
	AssignedByUserID *uuid.UUID `json:"assigned_by_user_id,omitempty"`
	AssignedAt       time.Time  `json:"assigned_at"`
	UnassignedAt     *time.Time `json:"unassigned_at,omitempty"`
}

// LineItemDetail mirrors the order_line_items fields required by detail views.
type LineItemDetail struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Category       string    `json:"category"`
	Strain         *string   `json:"strain,omitempty"`
	Classification *string   `json:"classification,omitempty"`
	Unit           string    `json:"unit"`
	UnitPriceCents int       `json:"unit_price_cents"`
	Qty            int       `json:"qty"`
	DiscountCents  int       `json:"discount_cents"`
	TotalCents     int       `json:"total_cents"`
	Status         string    `json:"status"`
	Notes          *string   `json:"notes,omitempty"`
}

// PaymentIntentDetail surfaces the payment intent fields needed on detail responses.
type PaymentIntentDetail struct {
	ID              uuid.UUID  `json:"id"`
	Method          string     `json:"method"`
	Status          string     `json:"status"`
	AmountCents     int        `json:"amount_cents"`
	CashCollectedAt *time.Time `json:"cash_collected_at,omitempty"`
	VendorPaidAt    *time.Time `json:"vendor_paid_at,omitempty"`
}

// OrderDetail bundles an order with its related preloads for detail rendering.
type OrderDetail struct {
	Order            *VendorOrderSummary     `json:"order"`
	LineItems        []LineItemDetail        `json:"line_items"`
	PaymentIntent    *PaymentIntentDetail    `json:"payment_intent,omitempty"`
	BuyerStore       OrderStoreSummary       `json:"buyer_store"`
	VendorStore      OrderStoreSummary       `json:"vendor_store"`
	ActiveAssignment *OrderAssignmentSummary `json:"active_assignment,omitempty"`
}
