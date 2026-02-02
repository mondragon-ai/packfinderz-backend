package types

import (
	"time"

	cbigquery "cloud.google.com/go/bigquery"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// MarketplaceEventRow mirrors the v2 marketplace_events BigQuery schema.
type MarketplaceEventRow struct {
	EventID             string             `bigquery:"event_id"`
	EventType           string             `bigquery:"event_type"`
	OccurredAt          time.Time          `bigquery:"occurred_at"`
	CheckoutGroupID     *string            `bigquery:"checkout_group_id"`
	OrderID             *string            `bigquery:"order_id"`
	BuyerStoreID        *string            `bigquery:"buyer_store_id"`
	VendorStoreID       *string            `bigquery:"vendor_store_id"`
	BuyerZip            *string            `bigquery:"buyer_zip"`
	BuyerLat            *float64           `bigquery:"buyer_lat"`
	BuyerLng            *float64           `bigquery:"buyer_lng"`
	SubtotalCents       *int64             `bigquery:"subtotal_cents"`
	DiscountsCents      *int64             `bigquery:"discounts_cents"`
	TaxCents            *int64             `bigquery:"tax_cents"`
	TransportFeeCents   *int64             `bigquery:"transport_fee_cents"`
	GrossRevenueCents   *int64             `bigquery:"gross_revenue_cents"`
	RefundCents         *int64             `bigquery:"refund_cents"`
	NetRevenueCents     *int64             `bigquery:"net_revenue_cents"`
	AttributedAdID      *string            `bigquery:"attributed_ad_id"`
	Items               cbigquery.NullJSON `bigquery:"items"`
	Payload             cbigquery.NullJSON `bigquery:"payload"`
	AttributedAdClickID *string            `bigquery:"attributed_ad_click_id"`
}

// AdEventFactRow mirrors the v2 ad_event_facts BigQuery schema.
type AdEventFactRow struct {
	EventID           string                `bigquery:"event_id"`
	OccurredAt        time.Time             `bigquery:"occurred_at"`
	AdID              string                `bigquery:"ad_id"`
	VendorStoreID     string                `bigquery:"vendor_store_id"`
	BuyerStoreID      *string               `bigquery:"buyer_store_id"`
	Type              enums.AdEventFactType `bigquery:"type"`
	CostCents         *int64                `bigquery:"cost_cents"`
	AttributedOrderID *string               `bigquery:"attributed_order_id"`
	BuyerZip          *string               `bigquery:"buyer_zip"`
	BuyerLat          *float64              `bigquery:"buyer_lat"`
	BuyerLng          *float64              `bigquery:"buyer_lng"`
	Payload           cbigquery.NullJSON    `bigquery:"payload"`
}
