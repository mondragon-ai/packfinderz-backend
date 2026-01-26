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
