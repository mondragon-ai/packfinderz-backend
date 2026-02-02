package types

import (
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// MarketplaceQueryRequest carries the input parameters for marketplace analytics queries.
type MarketplaceQueryRequest struct {
	StoreID   string
	StoreType enums.StoreType
	Start     time.Time
	End       time.Time
}

// TimeSeriesPoint describes a single date/value pair returned by the query service.
type TimeSeriesPoint struct {
	Date  string `json:"date"`
	Value int64  `json:"value"`
}

// LabelValue represents a top-N entry such as product/category/zip.
type LabelValue struct {
	Label string `json:"label"`
	Value int64  `json:"value"`
}

// MarketplaceQueryResponse wraps the marketplace KPIs for the dashboard.
type MarketplaceQueryResponse struct {
	OrdersSeries       []TimeSeriesPoint `json:"orders"`
	GrossRevenue       []TimeSeriesPoint `json:"gross_revenue"`
	DiscountsSeries    []TimeSeriesPoint `json:"discounts"`
	NetRevenue         []TimeSeriesPoint `json:"net_revenue"`
	TopProducts        []LabelValue      `json:"top_products"`
	TopCategories      []LabelValue      `json:"top_categories"`
	TopZIPs            []LabelValue      `json:"top_zips"`
	AOV                float64           `json:"aov"`
	NewCustomers       int64             `json:"new_customers"`
	ReturningCustomers int64             `json:"returning_customers"`
}
