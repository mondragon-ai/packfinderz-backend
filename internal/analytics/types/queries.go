package types

import "time"

// MarketplaceQueryRequest carries the input parameters for vendor analytics queries.
type MarketplaceQueryRequest struct {
	VendorStoreID string
	Start         time.Time
	End           time.Time
}

// MarketplaceKPIs summarize current vendor KPIs.
type MarketplaceKPIs struct {
	Orders             int64   `json:"orders"`
	RevenueCents       int64   `json:"revenue_cents"`
	AOVCents           float64 `json:"aov_cents"`
	CashCollectedCents int64   `json:"cash_collected_cents"`
}

// MarketplaceSeriesPoint describes a daily row returned by the vendor analytics query.
type MarketplaceSeriesPoint struct {
	Date               string `json:"date"`
	Orders             int64  `json:"orders"`
	RevenueCents       int64  `json:"revenue_cents"`
	CashCollectedCents int64  `json:"cash_collected_cents"`
}

// MarketplaceQueryResponse wraps KPIs and daily series data.
type MarketplaceQueryResponse struct {
	KPIs   MarketplaceKPIs          `json:"kpis"`
	Series []MarketplaceSeriesPoint `json:"series"`
}

// AdQueryRequest represents ad analytics request payloads.
type AdQueryRequest struct {
	VendorStoreID string
	Start         time.Time
	End           time.Time
}

// AdKPIs summarizes top-line ad metrics.
type AdKPIs struct {
	Impressions int64   `json:"impressions"`
	Clicks      int64   `json:"clicks"`
	Conversions int64   `json:"conversions"`
	SpendCents  int64   `json:"spend_cents"`
	ROAS        float64 `json:"roas"`
}

// AdSeriesPoint captures ad metric series data.
type AdSeriesPoint struct {
	Date        string `json:"date"`
	Impressions int64  `json:"impressions"`
	Clicks      int64  `json:"clicks"`
	Conversions int64  `json:"conversions"`
	SpendCents  int64  `json:"spend_cents"`
}

// AdQueryResponse holds ad KPI and series results.
type AdQueryResponse struct {
	KPIs   AdKPIs          `json:"kpis"`
	Series []AdSeriesPoint `json:"series"`
}
