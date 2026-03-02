package query

import (
	"context"
	"fmt"

	cloudbigquery "cloud.google.com/go/bigquery"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	pkgbigquery "github.com/angelmondragon/packfinderz-backend/pkg/bigquery"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"google.golang.org/api/iterator"
)

const (
	adMetricsSQL = `
SELECT
  COALESCE(SUM(CASE WHEN type = 'impression' THEN 1 ELSE 0 END), 0) AS impressions,
  COALESCE(SUM(CASE WHEN type = 'click' THEN 1 ELSE 0 END), 0) AS clicks,
  COALESCE(SUM(CASE WHEN type = 'conversion' THEN 1 ELSE 0 END), 0) AS conversions,
  COALESCE(SUM(CASE WHEN type = 'charge' THEN COALESCE(cost_cents, 0) ELSE 0 END), 0) AS spend_cents
FROM %s
WHERE vendor_store_id = @vendorStoreID
  AND ad_id = @adID
  AND occurred_at BETWEEN @start AND @end
`

	adRevenueSQL = `
SELECT
  COALESCE(SUM(COALESCE(gross_revenue_cents, 0)), 0) AS revenue_cents
FROM %s
WHERE vendor_store_id = @vendorStoreID
  AND attributed_ad_id = @adID
  AND occurred_at BETWEEN @start AND @end
`

	adSeriesSQL = `
WITH ad_metrics AS (
  SELECT
    DATE(occurred_at) AS day,
    COALESCE(SUM(CASE WHEN type = 'impression' THEN 1 ELSE 0 END), 0) AS impressions,
    COALESCE(SUM(CASE WHEN type = 'click' THEN 1 ELSE 0 END), 0) AS clicks,
    COALESCE(SUM(CASE WHEN type = 'conversion' THEN 1 ELSE 0 END), 0) AS conversions,
    COALESCE(SUM(CASE WHEN type = 'charge' THEN COALESCE(cost_cents, 0) ELSE 0 END), 0) AS spend_cents
  FROM %s
  WHERE vendor_store_id = @vendorStoreID
    AND ad_id = @adID
    AND occurred_at BETWEEN @start AND @end
  GROUP BY day
),
revenue AS (
  SELECT
    DATE(occurred_at) AS day,
    COALESCE(SUM(COALESCE(gross_revenue_cents, 0)), 0) AS revenue_cents
  FROM %s
  WHERE vendor_store_id = @vendorStoreID
    AND attributed_ad_id = @adID
    AND occurred_at BETWEEN @start AND @end
  GROUP BY day
),
buckets AS (
  SELECT day
  FROM UNNEST(GENERATE_DATE_ARRAY(DATE(@start), DATE(@end))) AS day
)
SELECT
  FORMAT_DATE("%%F", buckets.day) AS date,
  COALESCE(ad_metrics.impressions, 0) AS impressions,
  COALESCE(ad_metrics.clicks, 0) AS clicks,
  COALESCE(ad_metrics.conversions, 0) AS conversions,
  COALESCE(ad_metrics.spend_cents, 0) AS spend_cents,
  COALESCE(revenue.revenue_cents, 0) AS revenue_cents
FROM buckets
LEFT JOIN ad_metrics ON ad_metrics.day = buckets.day
LEFT JOIN revenue ON revenue.day = buckets.day
ORDER BY buckets.day ASC
`
)

// AdService exposes queries into the ad_event_facts + marketplace_events tables.
type AdService interface {
	Query(ctx context.Context, req types.AdQueryRequest) (*types.AdQueryResponse, error)
}

type adService struct {
	client           *pkgbigquery.Client
	adTable          string
	marketplaceTable string
}

// NewAdService builds an ad analytics service backed by BigQuery.
func NewAdService(client *pkgbigquery.Client, project, dataset, adTable, marketplaceTable string) (AdService, error) {
	if client == nil {
		return nil, fmt.Errorf("bigquery client required")
	}
	if project == "" || dataset == "" || adTable == "" || marketplaceTable == "" {
		return nil, fmt.Errorf("project, dataset, and table names are required")
	}
	return &adService{
		client:           client,
		adTable:          fmt.Sprintf("`%s.%s.%s`", project, dataset, adTable),
		marketplaceTable: fmt.Sprintf("`%s.%s.%s`", project, dataset, marketplaceTable),
	}, nil
}

func (s *adService) Query(ctx context.Context, req types.AdQueryRequest) (*types.AdQueryResponse, error) {
	if err := validateAdQueryRequest(req); err != nil {
		return nil, err
	}

	metrics, err := s.fetchMetrics(ctx, req)
	if err != nil {
		return nil, err
	}

	revenue, err := s.fetchRevenue(ctx, req)
	if err != nil {
		return nil, err
	}
	metrics.RevenueCents = revenue
	metrics.ROAS = computeRate(float64(revenue), float64(metrics.SpendCents))
	metrics.CPC = computeRate(float64(metrics.SpendCents), float64(metrics.Clicks))
	metrics.CPM = computeRate(float64(metrics.SpendCents), float64(metrics.Impressions)/1000)

	series, err := s.fetchSeries(ctx, req)
	if err != nil {
		return nil, err
	}

	return &types.AdQueryResponse{
		Metrics: metrics,
		Series:  series,
	}, nil
}

func (s *adService) fetchMetrics(ctx context.Context, req types.AdQueryRequest) (types.AdAnalyticsMetrics, error) {
	var metrics types.AdAnalyticsMetrics
	sql := fmt.Sprintf(adMetricsSQL, s.adTable)
	iter, err := s.client.Query(ctx, sql, commonParams(req))
	if err != nil {
		return metrics, fmt.Errorf("query ad metrics: %w", err)
	}

	var row struct {
		Impressions int64 `bigquery:"impressions"`
		Clicks      int64 `bigquery:"clicks"`
		Conversions int64 `bigquery:"conversions"`
		SpendCents  int64 `bigquery:"spend_cents"`
	}
	if err := iter.Next(&row); err != nil {
		if err == iterator.Done {
			return metrics, nil
		}
		return metrics, fmt.Errorf("reading metrics row: %w", err)
	}

	metrics.Impressions = row.Impressions
	metrics.Clicks = row.Clicks
	metrics.Conversions = row.Conversions
	metrics.SpendCents = row.SpendCents
	return metrics, nil
}

func (s *adService) fetchRevenue(ctx context.Context, req types.AdQueryRequest) (int64, error) {
	sql := fmt.Sprintf(adRevenueSQL, s.marketplaceTable)
	iter, err := s.client.Query(ctx, sql, commonParams(req))
	if err != nil {
		return 0, fmt.Errorf("query ad revenue: %w", err)
	}

	var row struct {
		RevenueCents int64 `bigquery:"revenue_cents"`
	}
	if err := iter.Next(&row); err != nil {
		if err == iterator.Done {
			return 0, nil
		}
		return 0, fmt.Errorf("reading revenue row: %w", err)
	}
	return row.RevenueCents, nil
}

func (s *adService) fetchSeries(ctx context.Context, req types.AdQueryRequest) ([]types.AdAnalyticsSeriesPoint, error) {
	sql := fmt.Sprintf(adSeriesSQL, s.adTable, s.marketplaceTable)
	iter, err := s.client.Query(ctx, sql, commonParams(req))
	if err != nil {
		return nil, fmt.Errorf("query ad series: %w", err)
	}

	series := make([]types.AdAnalyticsSeriesPoint, 0)
	for {
		var row types.AdAnalyticsSeriesPoint
		if err := iter.Next(&row); err != nil {
			if err == iterator.Done {
				break
			}
			return nil, fmt.Errorf("reading series row: %w", err)
		}
		series = append(series, row)
	}

	return series, nil
}

func commonParams(req types.AdQueryRequest) []cloudbigquery.QueryParameter {
	return []cloudbigquery.QueryParameter{
		{Name: "vendorStoreID", Value: req.VendorStoreID},
		{Name: "adID", Value: req.AdID},
		{Name: "start", Value: req.Start.UTC()},
		{Name: "end", Value: req.End.UTC()},
	}
}

func validateAdQueryRequest(req types.AdQueryRequest) error {
	if req.VendorStoreID == "" {
		return pkgerrors.New(pkgerrors.CodeValidation, "vendor_store_id is required")
	}
	if req.AdID == "" {
		return pkgerrors.New(pkgerrors.CodeValidation, "ad_id is required")
	}
	if req.Start.IsZero() || req.End.IsZero() {
		return pkgerrors.New(pkgerrors.CodeValidation, "start and end are required")
	}
	if req.End.Before(req.Start) {
		return pkgerrors.New(pkgerrors.CodeValidation, "end must be after start")
	}
	return nil
}

func computeRate(numerator, denominator float64) float64 {
	if denominator <= 0 {
		return 0
	}
	return numerator / denominator
}
