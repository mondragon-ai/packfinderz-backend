package payloads

import "time"

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
