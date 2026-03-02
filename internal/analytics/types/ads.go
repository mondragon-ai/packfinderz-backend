package types

import "time"

// AdQueryRequest carries the inputs required to fetch ad analytics for a timeframe.
type AdQueryRequest struct {
	VendorStoreID string
	AdID          string
	Start         time.Time
	End           time.Time
}

// AdAnalyticsMetrics exposes top-line metrics for an ad over a timeframe.
type AdAnalyticsMetrics struct {
	Impressions  int64   `json:"impressions"`
	Clicks       int64   `json:"clicks"`
	Conversions  int64   `json:"conversions"`
	SpendCents   int64   `json:"spend_cents"`
	RevenueCents int64   `json:"revenue_cents"`
	ROAS         float64 `json:"roas"`
	CPC          float64 `json:"cpc"`
	CPM          float64 `json:"cpm"`
}

// AdAnalyticsSeriesPoint describes per-day metrics for an ad report.
type AdAnalyticsSeriesPoint struct {
	Date         string `json:"date"`
	Impressions  int64  `json:"impressions"`
	Clicks       int64  `json:"clicks"`
	Conversions  int64  `json:"conversions"`
	SpendCents   int64  `json:"spend_cents"`
	RevenueCents int64  `json:"revenue_cents"`
}

// AdQueryResponse wraps the metrics + time series payload delivered to clients.
type AdQueryResponse struct {
	Metrics AdAnalyticsMetrics       `json:"metrics"`
	Series  []AdAnalyticsSeriesPoint `json:"series"`
}
