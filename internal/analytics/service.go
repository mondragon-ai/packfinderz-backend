package analytics

import (
	"context"
	"fmt"

	cloudbigquery "cloud.google.com/go/bigquery"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	pkgbigquery "github.com/angelmondragon/packfinderz-backend/pkg/bigquery"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"google.golang.org/api/iterator"
)

// Service provides analytics reports based on BigQuery data.
type Service interface {
	// VendorAnalytics returns KPIs and daily aggregates for the specified vendor store within the requested time range.
	VendorAnalytics(ctx context.Context, req types.MarketplaceQueryRequest) (*types.MarketplaceQueryResponse, error)
}

type service struct {
	client   *pkgbigquery.Client
	tableRef string
}

// NewService builds an analytics service backed by BigQuery.
func NewService(client *pkgbigquery.Client, project, dataset, table string) (Service, error) {
	if client == nil {
		return nil, fmt.Errorf("bigquery client required")
	}
	if project == "" {
		return nil, fmt.Errorf("gcp project is required")
	}
	if dataset == "" {
		return nil, fmt.Errorf("bigquery dataset is required")
	}
	if table == "" {
		return nil, fmt.Errorf("bigquery table is required")
	}
	tableRef := fmt.Sprintf("`%s.%s.%s`", project, dataset, table)
	return &service{
		client:   client,
		tableRef: tableRef,
	}, nil
}

func (s *service) VendorAnalytics(ctx context.Context, req types.MarketplaceQueryRequest) (*types.MarketplaceQueryResponse, error) {
	if req.VendorStoreID == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "vendor store id required")
	}
	if req.End.Before(req.Start) {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "invalid time range")
	}

	params := queryParameters(req)

	kpis, err := s.fetchKPIs(ctx, params)
	if err != nil {
		return nil, err
	}
	series, err := s.fetchSeries(ctx, params)
	if err != nil {
		return nil, err
	}

	return &types.MarketplaceQueryResponse{
		KPIs:   kpis,
		Series: series,
	}, nil
}

func queryParameters(req types.MarketplaceQueryRequest) []cloudbigquery.QueryParameter {
	return []cloudbigquery.QueryParameter{
		{Name: "vendorStoreID", Value: req.VendorStoreID},
		{Name: "start", Value: req.Start},
		{Name: "end", Value: req.End},
	}
}

func (s *service) fetchKPIs(ctx context.Context, params []cloudbigquery.QueryParameter) (types.MarketplaceKPIs, error) {
	const sql = `
WITH base AS (
  SELECT
    COUNTIF(event_type = 'order_created') AS orders,
    SUMIF(event_type = 'order_paid', SAFE_CAST(JSON_VALUE(payload, '$.amount_cents') AS INT64)) AS revenue_cents,
    SUMIF(event_type = 'cash_collected', SAFE_CAST(JSON_VALUE(payload, '$.amount_cents') AS INT64)) AS cash_collected_cents
  FROM %s
  WHERE vendor_store_id = @vendorStoreID
    AND occurred_at BETWEEN @start AND @end
)
SELECT
  orders,
  revenue_cents,
  SAFE_DIVIDE(revenue_cents, NULLIF(orders, 0)) AS aov_cents,
  cash_collected_cents
FROM base
`
	iter, err := s.client.Query(ctx, fmt.Sprintf(sql, s.tableRef), params)
	if err != nil {
		return types.MarketplaceKPIs{}, fmt.Errorf("kpi query failed: %w", err)
	}

	var row struct {
		Orders             int64                     `bigquery:"orders"`
		RevenueCents       cloudbigquery.NullInt64   `bigquery:"revenue_cents"`
		AOVCents           cloudbigquery.NullFloat64 `bigquery:"aov_cents"`
		CashCollectedCents cloudbigquery.NullInt64   `bigquery:"cash_collected_cents"`
	}
	if err := iter.Next(&row); err != nil {
		if err == iterator.Done {
			return types.MarketplaceKPIs{}, nil
		}
		return types.MarketplaceKPIs{}, fmt.Errorf("reading kpi row: %w", err)
	}

	return types.MarketplaceKPIs{
		Orders:             row.Orders,
		RevenueCents:       int64Value(row.RevenueCents),
		AOVCents:           float64Value(row.AOVCents),
		CashCollectedCents: int64Value(row.CashCollectedCents),
	}, nil
}

func (s *service) fetchSeries(ctx context.Context, params []cloudbigquery.QueryParameter) ([]types.MarketplaceSeriesPoint, error) {
	const sql = `
SELECT
  DATE_TRUNC(occurred_at, DAY) AS day,
  COUNTIF(event_type = 'order_created') AS orders,
  SUMIF(event_type = 'order_paid', SAFE_CAST(JSON_VALUE(payload, '$.amount_cents') AS INT64)) AS revenue_cents,
  SUMIF(event_type = 'cash_collected', SAFE_CAST(JSON_VALUE(payload, '$.amount_cents') AS INT64)) AS cash_collected_cents
FROM %s
WHERE vendor_store_id = @vendorStoreID
  AND occurred_at BETWEEN @start AND @end
GROUP BY day
ORDER BY day ASC
`
	iter, err := s.client.Query(ctx, fmt.Sprintf(sql, s.tableRef), params)
	if err != nil {
		return nil, fmt.Errorf("series query failed: %w", err)
	}

	var points []types.MarketplaceSeriesPoint
	for {
		var row struct {
			Day                cloudbigquery.NullDate  `bigquery:"day"`
			Orders             int64                   `bigquery:"orders"`
			RevenueCents       cloudbigquery.NullInt64 `bigquery:"revenue_cents"`
			CashCollectedCents cloudbigquery.NullInt64 `bigquery:"cash_collected_cents"`
		}
		err := iter.Next(&row)
		if err != nil {
			if err == iterator.Done {
				break
			}
			return nil, fmt.Errorf("reading series row: %w", err)
		}
		date := ""
		if row.Day.Valid {
			date = row.Day.Date.String()
		}
		points = append(points, types.MarketplaceSeriesPoint{
			Date:               date,
			Orders:             row.Orders,
			RevenueCents:       int64Value(row.RevenueCents),
			CashCollectedCents: int64Value(row.CashCollectedCents),
		})
	}

	return points, nil
}

func int64Value(n cloudbigquery.NullInt64) int64 {
	if n.Valid {
		return n.Int64
	}
	return 0
}

func float64Value(n cloudbigquery.NullFloat64) float64 {
	if n.Valid {
		return n.Float64
	}
	return 0
}
