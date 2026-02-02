package query

import (
	"context"
	"fmt"

	cloudbigquery "cloud.google.com/go/bigquery"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/bigquery"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"google.golang.org/api/iterator"
)

const (
	timeSeriesOrdersSQL = `
SELECT
  FORMAT_DATE('%%F', DATE_TRUNC(occurred_at, DAY)) AS day,
  COUNTIF(event_type = 'order_created') AS value
FROM %s
WHERE %s
  AND event_type = 'order_created'
  AND occurred_at BETWEEN @start AND @end
GROUP BY day
ORDER BY day ASC
`

	timeSeriesRevenueSQL = `
SELECT
  FORMAT_DATE('%%F', DATE_TRUNC(occurred_at, DAY)) AS day,
  SUM(COALESCE(%s, 0)) AS value
FROM %s
WHERE %s
  AND event_type IN ('order_paid', 'cash_collected')
  AND occurred_at BETWEEN @start AND @end
GROUP BY day
ORDER BY day ASC
`

	timeSeriesDiscountsSQL = `
SELECT
  FORMAT_DATE('%%F', DATE_TRUNC(occurred_at, DAY)) AS day,
  SUM(COALESCE(discounts_cents, 0)) AS value
FROM %s
WHERE %s
  AND event_type = 'order_created'
  AND occurred_at BETWEEN @start AND @end
GROUP BY day
ORDER BY day ASC
`

	topProductsSQL = `
SELECT label, SUM(value) AS value FROM (
  SELECT
    JSON_VALUE(item, '$.product_id') AS label,
    SAFE_CAST(JSON_VALUE(item, '$.line_total_cents') AS INT64) AS value
  FROM %s
  WHERE %s
    AND items IS NOT NULL
    AND event_type = 'order_created'
    AND occurred_at BETWEEN @start AND @end,
  UNNEST(JSON_EXTRACT_ARRAY(items)) AS item
)
WHERE label IS NOT NULL
GROUP BY label
ORDER BY value DESC
LIMIT 5
`

	topCategoriesSQL = `
SELECT label, SUM(value) AS value FROM (
  SELECT
    JSON_VALUE(item, '$.category') AS label,
    SAFE_CAST(JSON_VALUE(item, '$.line_total_cents') AS INT64) AS value
  FROM %s
  WHERE %s
    AND items IS NOT NULL
    AND event_type = 'order_created'
    AND occurred_at BETWEEN @start AND @end,
  UNNEST(JSON_EXTRACT_ARRAY(items)) AS item
)
WHERE label IS NOT NULL
GROUP BY label
ORDER BY value DESC
LIMIT 5
`

	topZipsSQL = `
SELECT buyer_zip AS label, SUM(COALESCE(gross_revenue_cents, 0)) AS value
FROM %s
WHERE %s
  AND buyer_zip IS NOT NULL
  AND event_type IN ('order_paid', 'cash_collected')
  AND occurred_at BETWEEN @start AND @end
GROUP BY buyer_zip
ORDER BY value DESC
LIMIT 5
`

	aovSQL = `
SELECT SAFE_DIVIDE(SUM(COALESCE(gross_revenue_cents, 0)), NULLIF(COUNT(DISTINCT order_id), 0)) AS value
FROM %s
WHERE %s
  AND event_type IN ('order_paid', 'cash_collected')
  AND occurred_at BETWEEN @start AND @end
`

	newReturningSQL = `
WITH prior_buyers AS (
  SELECT DISTINCT buyer_store_id
  FROM %s
  WHERE %s
    AND event_type IN ('order_paid', 'cash_collected')
    AND occurred_at < @start
    AND buyer_store_id IS NOT NULL
),
current_buyers AS (
  SELECT DISTINCT buyer_store_id,
    CASE
      WHEN buyer_store_id IN (SELECT buyer_store_id FROM prior_buyers) THEN 'returning'
      ELSE 'new'
    END AS category
  FROM %s
  WHERE %s
    AND event_type IN ('order_paid', 'cash_collected')
    AND occurred_at BETWEEN @start AND @end
    AND buyer_store_id IS NOT NULL
)
SELECT
  COUNTIF(category = 'new') AS new_customers,
  COUNTIF(category = 'returning') AS returning_customers
FROM current_buyers
`
)

// MarketplaceService provides dashboard data from BigQuery marketplace_events.
type MarketplaceService interface {
	Query(ctx context.Context, req types.MarketplaceQueryRequest) (*types.MarketplaceQueryResponse, error)
}

type marketplaceService struct {
	client   *bigquery.Client
	tableRef string
}

// NewMarketplaceService builds a service backed by BigQuery.
func NewMarketplaceService(client *bigquery.Client, project, dataset, table string) (MarketplaceService, error) {
	if client == nil {
		return nil, fmt.Errorf("bigquery client required")
	}
	if project == "" || dataset == "" || table == "" {
		return nil, fmt.Errorf("project, dataset, and table are required")
	}
	return &marketplaceService{
		client:   client,
		tableRef: fmt.Sprintf("`%s.%s.%s`", project, dataset, table),
	}, nil
}

func (s *marketplaceService) Query(ctx context.Context, req types.MarketplaceQueryRequest) (*types.MarketplaceQueryResponse, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	storeClause, err := buildStoreClause(req.StoreType)
	if err != nil {
		return nil, err
	}
	params := s.baseParams(req)

	orders, err := s.querySeries(ctx, fmt.Sprintf(timeSeriesOrdersSQL, s.tableRef, storeClause), params)
	if err != nil {
		return nil, err
	}
	grossRevenue, err := s.querySeries(ctx, fmt.Sprintf(timeSeriesRevenueSQL, "gross_revenue_cents", s.tableRef, storeClause), params)
	if err != nil {
		return nil, err
	}
	netRevenue, err := s.querySeries(ctx, fmt.Sprintf(timeSeriesRevenueSQL, "net_revenue_cents", s.tableRef, storeClause), params)
	if err != nil {
		return nil, err
	}
	discounts, err := s.querySeries(ctx, fmt.Sprintf(timeSeriesDiscountsSQL, s.tableRef, storeClause), params)
	if err != nil {
		return nil, err
	}

	topProducts, err := s.queryTopLabels(ctx, fmt.Sprintf(topProductsSQL, s.tableRef, storeClause), params)
	if err != nil {
		return nil, err
	}
	topCategories, err := s.queryTopLabels(ctx, fmt.Sprintf(topCategoriesSQL, s.tableRef, storeClause), params)
	if err != nil {
		return nil, err
	}
	topZIPs, err := s.queryTopLabels(ctx, fmt.Sprintf(topZipsSQL, s.tableRef, storeClause), params)
	if err != nil {
		return nil, err
	}

	aov, err := s.queryAOV(ctx, fmt.Sprintf(aovSQL, s.tableRef, storeClause), params)
	if err != nil {
		return nil, err
	}

	newCustomers, returningCustomers, err := s.queryNewReturning(ctx, fmt.Sprintf(newReturningSQL, s.tableRef, storeClause, s.tableRef, storeClause), params)
	if err != nil {
		return nil, err
	}

	return &types.MarketplaceQueryResponse{
		OrdersSeries:       orders,
		GrossRevenue:       grossRevenue,
		DiscountsSeries:    discounts,
		NetRevenue:         netRevenue,
		TopProducts:        topProducts,
		TopCategories:      topCategories,
		TopZIPs:            topZIPs,
		AOV:                aov,
		NewCustomers:       newCustomers,
		ReturningCustomers: returningCustomers,
	}, nil
}

func validateRequest(req types.MarketplaceQueryRequest) error {
	if req.StoreID == "" {
		return pkgerrors.New(pkgerrors.CodeValidation, "store id required")
	}
	if !req.StoreType.IsValid() {
		return pkgerrors.New(pkgerrors.CodeValidation, "store type required")
	}
	if req.Start.IsZero() || req.End.IsZero() {
		return pkgerrors.New(pkgerrors.CodeValidation, "start and end are required")
	}
	if req.End.Before(req.Start) {
		return pkgerrors.New(pkgerrors.CodeValidation, "end must be after start")
	}
	return nil
}

func buildStoreClause(storeType enums.StoreType) (string, error) {
	switch storeType {
	case enums.StoreTypeVendor:
		return "vendor_store_id = @storeID", nil
	case enums.StoreTypeBuyer:
		return "buyer_store_id = @storeID", nil
	default:
		return "", pkgerrors.New(pkgerrors.CodeValidation, "invalid store type")
	}
}

func (s *marketplaceService) baseParams(req types.MarketplaceQueryRequest) []cloudbigquery.QueryParameter {
	return []cloudbigquery.QueryParameter{
		{Name: "storeID", Value: req.StoreID},
		{Name: "start", Value: req.Start},
		{Name: "end", Value: req.End},
	}
}

func (s *marketplaceService) querySeries(ctx context.Context, sql string, params []cloudbigquery.QueryParameter) ([]types.TimeSeriesPoint, error) {
	iter, err := s.client.Query(ctx, sql, params)
	if err != nil {
		return nil, fmt.Errorf("query series: %w", err)
	}

	var points []types.TimeSeriesPoint
	for {
		var row struct {
			Day   string `bigquery:"day"`
			Value int64  `bigquery:"value"`
		}
		if err := iter.Next(&row); err != nil {
			if err == iterator.Done {
				break
			}
			return nil, fmt.Errorf("reading series row: %w", err)
		}
		points = append(points, types.TimeSeriesPoint{Date: row.Day, Value: row.Value})
	}
	return points, nil
}

func (s *marketplaceService) queryTopLabels(ctx context.Context, sql string, params []cloudbigquery.QueryParameter) ([]types.LabelValue, error) {
	iter, err := s.client.Query(ctx, sql, params)
	if err != nil {
		return nil, fmt.Errorf("query top labels: %w", err)
	}

	var result []types.LabelValue
	for {
		var row struct {
			Label string `bigquery:"label"`
			Value int64  `bigquery:"value"`
		}
		if err := iter.Next(&row); err != nil {
			if err == iterator.Done {
				break
			}
			return nil, fmt.Errorf("reading top label row: %w", err)
		}
		result = append(result, types.LabelValue{Label: row.Label, Value: row.Value})
	}
	return result, nil
}

func (s *marketplaceService) queryAOV(ctx context.Context, sql string, params []cloudbigquery.QueryParameter) (float64, error) {
	iter, err := s.client.Query(ctx, sql, params)
	if err != nil {
		return 0, fmt.Errorf("query aov: %w", err)
	}
	var row struct {
		Value cloudbigquery.NullFloat64 `bigquery:"value"`
	}
	if err := iter.Next(&row); err != nil {
		if err == iterator.Done {
			return 0, nil
		}
		return 0, fmt.Errorf("reading aov row: %w", err)
	}
	if !row.Value.Valid {
		return 0, nil
	}
	return row.Value.Float64, nil
}

func (s *marketplaceService) queryNewReturning(ctx context.Context, sql string, params []cloudbigquery.QueryParameter) (int64, int64, error) {
	iter, err := s.client.Query(ctx, sql, params)
	if err != nil {
		return 0, 0, fmt.Errorf("query new vs returning: %w", err)
	}
	var row struct {
		NewCustomers       int64 `bigquery:"new_customers"`
		ReturningCustomers int64 `bigquery:"returning_customers"`
	}
	if err := iter.Next(&row); err != nil {
		if err == iterator.Done {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("reading new vs returning row: %w", err)
	}
	return row.NewCustomers, row.ReturningCustomers, nil
}
