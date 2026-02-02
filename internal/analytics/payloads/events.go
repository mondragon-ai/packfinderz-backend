package payloads

import "time"

// OrderCreatedEvent matches the canonical analytics payload derived from a vendor order snapshot.
type OrderCreatedEvent struct {
	CheckoutGroupID     string             `json:"checkout_group_id"`
	CartID              string             `json:"cart_id"`
	OrderID             string             `json:"order_id"`
	BuyerStoreID        string             `json:"buyer_store_id"`
	VendorStoreID       string             `json:"vendor_store_id"`
	Currency            string             `json:"currency"`
	SubtotalCents       int64              `json:"subtotal_cents"`
	DiscountsCents      int64              `json:"discounts_cents"`
	TaxCents            int64              `json:"tax_cents"`
	TransportFeeCents   int64              `json:"transport_fee_cents"`
	TotalCents          int64              `json:"total_cents"`
	ShippingAddress     *ShippingAddress   `json:"shipping_address"`
	Items               []OrderCreatedItem `json:"items"`
	OrderSnapshotStatus string             `json:"order_snapshot_status"`
	AttributedAdTokens  []string           `json:"attributed_ad_tokens"`
}

// ShippingAddress mirrors the canonical subset used by analytics.
type ShippingAddress struct {
	PostalCode string  `json:"postal_code"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
}

// OrderCreatedItem captures the per-line snapshot recorded at checkout.
type OrderCreatedItem struct {
	ProductID         string   `json:"product_id"`
	Title             string   `json:"title"`
	Category          string   `json:"category"`
	Classification    string   `json:"classification"`
	Qty               int64    `json:"qty"`
	Moq               int64    `json:"moq"`
	MaxQty            *int64   `json:"max_qty"`
	UnitPriceCents    int64    `json:"unit_price_cents"`
	LineSubtotalCents int64    `json:"line_subtotal_cents"`
	LineTotalCents    int64    `json:"line_total_cents"`
	DiscountCents     int64    `json:"discount_cents"`
	Status            string   `json:"status"`
	Warnings          []string `json:"warnings"`
	AttributedAdID    *string  `json:"attributed_ad_id"`
}

// CashCollectedEvent mirrors the canonical cash_collected payload.
type CashCollectedEvent struct {
	OrderID         string    `json:"order_id"`
	BuyerStoreID    string    `json:"buyer_store_id"`
	VendorStoreID   string    `json:"vendor_store_id"`
	AmountCents     int       `json:"amount_cents"`
	CashCollectedAt time.Time `json:"cash_collected_at"`
}

// AdImpressionEvent describes the tracked ad impression.
type AdImpressionEvent struct {
	AdID          string   `json:"ad_id"`
	VendorStoreID string   `json:"vendor_store_id"`
	BuyerStoreID  *string  `json:"buyer_store_id,omitempty"`
	BuyerUserID   *string  `json:"buyer_user_id,omitempty"`
	OccurredAt    string   `json:"occurred_at"`
	BuyerZip      *string  `json:"buyer_zip,omitempty"`
	BuyerLat      *float64 `json:"buyer_lat,omitempty"`
	BuyerLng      *float64 `json:"buyer_lng,omitempty"`
}

// AdClickEvent describes the tracked ad click.
type AdClickEvent struct {
	AdID          string   `json:"ad_id"`
	VendorStoreID string   `json:"vendor_store_id"`
	BuyerStoreID  *string  `json:"buyer_store_id,omitempty"`
	BuyerUserID   *string  `json:"buyer_user_id,omitempty"`
	OccurredAt    string   `json:"occurred_at"`
	BuyerZip      *string  `json:"buyer_zip,omitempty"`
	BuyerLat      *float64 `json:"buyer_lat,omitempty"`
	BuyerLng      *float64 `json:"buyer_lng,omitempty"`
}

// AdDailyChargeRecordedEvent describes the nightly charge for an ad.
type AdDailyChargeRecordedEvent struct {
	AdID          string `json:"ad_id"`
	VendorStoreID string `json:"vendor_store_id"`
	CostCents     int64  `json:"cost_cents"`
	OccurredAt    string `json:"occurred_at"`
}
