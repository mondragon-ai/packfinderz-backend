// internal/analytics/query/marketplace_timeseries_buckets.go
//
// Bucketed (zero-filled) marketplace time series queries for:
// - orders (order_created count)
// - gross_revenue (gross_revenue_cents sum for paid/collected)
// - net_revenue (net_revenue_cents sum for paid/collected)
// - discounts (discounts_cents sum for order_created)
//
// IMPORTANT: This file is designed to NOT break your current endpoint response.
// It returns []types.TimeSeriesPoint using the existing `{ date, value }` shape,
// and adds only an OPTIONAL `bucket_interval` string if you choose to include it
// at the response layer.
//
// Timezone: fixed to America/Chicago (US Central) for now, per your request.
//
// How to wire (minimal change):
// - In marketplaceService.Query(...), replace the 4 separate s.querySeries(...) calls
//   with s.queryBucketedSeries(...) from this file, then assign results to the same
//   response fields (OrdersSeries, GrossRevenue, NetRevenue, DiscountsSeries).
//
// Notes:
// - Buckets are zero-filled using GENERATE_TIMESTAMP_ARRAY / GENERATE_DATE_ARRAY.
// - The `date` string returned is an ISO-ish bucket key suitable for sorting.
//   hour: "YYYY-MM-DDTHH:00:00-06:00" (or -05:00 in DST) because we format in tz
//   day/week: "YYYY-MM-DD"
//   month: "YYYY-MM"
//   year: "YYYY"

package query

import (
	"context"
	"fmt"
	"time"

	cloudbigquery "cloud.google.com/go/bigquery"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"google.golang.org/api/iterator"
)

const (
	centralTZ = "America/Chicago"
)

// returns one of: "hour","day","week","month","year"
func selectGranularity(start, end time.Time) string {
	d := end.Sub(start)
	switch {
	case d <= 24*time.Hour:
		return "hour" // today or very short ranges
	case d <= 7*24*time.Hour:
		return "hour" // 7 days -> hourly
	case d <= 90*24*time.Hour:
		return "day" // 30 & 90 days -> daily
	case d <= 365*24*time.Hour/2: // <= ~6 months
		return "week"
	case d <= 365*24*time.Hour:
		return "week" // 12 months -> weekly
	case d <= 2*365*24*time.Hour:
		return "week" // 1-2 years -> weekly
	case d <= 6*365*24*time.Hour:
		return "month" // 2-6 years -> monthly
	default:
		return "year" // >= 6 years -> yearly
	}
}

type bucketedSeriesResult struct {
	Granularity string
	Orders      []types.TimeSeriesPoint
	Gross       []types.TimeSeriesPoint
	Net         []types.TimeSeriesPoint
	Discounts   []types.TimeSeriesPoint
}

func validateGranularity(g string) error {
	switch g {
	case "hour", "day", "week", "month", "year":
		return nil
	default:
		return errors.New(errors.CodeValidation, "invalid granularity")
	}
}

// queryBucketedSeries runs ONE BigQuery query (per requested granularity) that returns
// all 4 time series with zero-fill, then splits it into your existing DTO slices.
//
// - tableRef: formatted like `project.dataset.table` (already quoted with backticks upstream)
// - storeClause: e.g. "vendor_store_id = @storeID" or "buyer_store_id = @storeID"
func (s *marketplaceService) queryBucketedSeries(
	ctx context.Context,
	tableRef string,
	storeClause string,
	start time.Time,
	end time.Time,
	storeID string,
	storeType enums.StoreType,
) (*bucketedSeriesResult, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("bigquery client required")
	}
	if tableRef == "" || storeClause == "" {
		return nil, fmt.Errorf("tableRef and storeClause required")
	}
	if storeID == "" || !storeType.IsValid() {
		return nil, errors.New(errors.CodeValidation, "store context required")
	}
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return nil, errors.New(errors.CodeValidation, "invalid time range")
	}

	g := selectGranularity(start, end)
	if err := validateGranularity(g); err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(bucketedMarketplaceSeriesSQL(g), tableRef, storeClause)

	params := []cloudbigquery.QueryParameter{
		{Name: "storeID", Value: storeID},
		{Name: "start", Value: start.UTC()},
		{Name: "end", Value: end.UTC()},
	}

	iter, err := s.client.Query(ctx, sql, params)
	if err != nil {
		return nil, fmt.Errorf("query bucketed series: %w", err)
	}

	out := &bucketedSeriesResult{
		Granularity: g,
		Orders:      []types.TimeSeriesPoint{},
		Gross:       []types.TimeSeriesPoint{},
		Net:         []types.TimeSeriesPoint{},
		Discounts:   []types.TimeSeriesPoint{},
	}

	for {
		var row struct {
			Date              string `bigquery:"date"`
			Orders            int64  `bigquery:"orders"`
			GrossRevenueCents int64  `bigquery:"gross_revenue_cents"`
			NetRevenueCents   int64  `bigquery:"net_revenue_cents"`
			DiscountsCents    int64  `bigquery:"discounts_cents"`
		}
		if err := iter.Next(&row); err != nil {
			if err == iterator.Done {
				break
			}
			return nil, fmt.Errorf("reading bucketed series row: %w", err)
		}

		out.Orders = append(out.Orders, types.TimeSeriesPoint{Date: row.Date, Value: row.Orders})
		out.Gross = append(out.Gross, types.TimeSeriesPoint{Date: row.Date, Value: row.GrossRevenueCents})
		out.Net = append(out.Net, types.TimeSeriesPoint{Date: row.Date, Value: row.NetRevenueCents})
		out.Discounts = append(out.Discounts, types.TimeSeriesPoint{Date: row.Date, Value: row.DiscountsCents})
	}

	return out, nil
}

func bucketedMarketplaceSeriesSQL(granularity string) string {
	switch granularity {
	case "hour":
		return bucketedHourlySQL
	case "day":
		return bucketedDailySQL
	case "week":
		return bucketedWeeklySQL
	case "month":
		return bucketedMonthlySQL
	case "year":
		return bucketedYearlySQL
	default:
		// validateGranularity should prevent this
		return bucketedDailySQL
	}
}

// =========================
// SQL TEMPLATES (zero-fill)
// =========================
//
// Each query returns rows with columns:
// - date (string key formatted in America/Chicago)
// - orders (int64)
// - gross_revenue_cents (int64)
// - net_revenue_cents (int64)
// - discounts_cents (int64)
//
// Replace placeholders:
// - %s = tableRef (already wrapped in backticks upstream)
// - %s = storeClause (uses @storeID param)

const bucketedHourlySQL = `
WITH buckets AS (
  SELECT ts AS bucket_ts
  FROM UNNEST(
    GENERATE_TIMESTAMP_ARRAY(@start, @end, INTERVAL 1 HOUR)
  ) AS ts
),
agg AS (
  SELECT
    TIMESTAMP_TRUNC(occurred_at, HOUR, "` + centralTZ + `") AS bucket_ts,

    COUNTIF(event_type = 'order_created') AS orders,

    SUM(
      CASE
        WHEN event_type IN ('order_paid', 'cash_collected') THEN COALESCE(gross_revenue_cents, 0)
        ELSE 0
      END
    ) AS gross_revenue_cents,

    SUM(
      CASE
        WHEN event_type IN ('order_paid', 'cash_collected') THEN COALESCE(net_revenue_cents, 0)
        ELSE 0
      END
    ) AS net_revenue_cents,

    SUM(
      CASE
        WHEN event_type = 'order_created' THEN COALESCE(discounts_cents, 0)
        ELSE 0
      END
    ) AS discounts_cents

  FROM %s
  WHERE %s
    AND occurred_at BETWEEN @start AND @end
  GROUP BY bucket_ts
)
SELECT
  -- RFC3339-ish local timestamp for sorting + labeling (includes CST/CDT offset)
  FORMAT_TIMESTAMP('%%Y-%%m-%%dT%%H:00:00%%Ez', b.bucket_ts, "America/Chicago") AS date,
  COALESCE(a.orders, 0) AS orders,
  COALESCE(a.gross_revenue_cents, 0) AS gross_revenue_cents,
  COALESCE(a.net_revenue_cents, 0) AS net_revenue_cents,
  COALESCE(a.discounts_cents, 0) AS discounts_cents
FROM buckets b
LEFT JOIN agg a USING(bucket_ts)
ORDER BY b.bucket_ts ASC
`

const bucketedDailySQL = `
WITH buckets AS (
  SELECT d AS bucket_day
  FROM UNNEST(
    GENERATE_DATE_ARRAY(
      DATE(@start, "` + centralTZ + `"),
      DATE(@end, "` + centralTZ + `"),
      INTERVAL 1 DAY
    )
  ) AS d
),
agg AS (
  SELECT
    DATE(TIMESTAMP_TRUNC(occurred_at, DAY, "` + centralTZ + `")) AS bucket_day,

    COUNTIF(event_type = 'order_created') AS orders,

    SUM(
      CASE
        WHEN event_type IN ('order_paid', 'cash_collected') THEN COALESCE(gross_revenue_cents, 0)
        ELSE 0
      END
    ) AS gross_revenue_cents,

    SUM(
      CASE
        WHEN event_type IN ('order_paid', 'cash_collected') THEN COALESCE(net_revenue_cents, 0)
        ELSE 0
      END
    ) AS net_revenue_cents,

    SUM(
      CASE
        WHEN event_type = 'order_created' THEN COALESCE(discounts_cents, 0)
        ELSE 0
      END
    ) AS discounts_cents

  FROM %s
  WHERE %s
    AND occurred_at BETWEEN @start AND @end
  GROUP BY bucket_day
)
SELECT
  FORMAT_DATE('%%F', b.bucket_day) AS date, -- YYYY-MM-DD
  COALESCE(a.orders, 0) AS orders,
  COALESCE(a.gross_revenue_cents, 0) AS gross_revenue_cents,
  COALESCE(a.net_revenue_cents, 0) AS net_revenue_cents,
  COALESCE(a.discounts_cents, 0) AS discounts_cents
FROM buckets b
LEFT JOIN agg a USING(bucket_day)
ORDER BY b.bucket_day ASC
`

const bucketedWeeklySQL = `
WITH buckets AS (
  SELECT d AS bucket_day
  FROM UNNEST(
    GENERATE_DATE_ARRAY(
      DATE(@start, "` + centralTZ + `"),
      DATE(@end, "` + centralTZ + `"),
      INTERVAL 1 WEEK
    )
  ) AS d
),
agg AS (
  SELECT
    -- Week bucket = Monday-start (local)
    DATE_TRUNC(DATE(occurred_at, "` + centralTZ + `"), WEEK(MONDAY)) AS bucket_day,

    COUNTIF(event_type = 'order_created') AS orders,

    SUM(
      CASE
        WHEN event_type IN ('order_paid', 'cash_collected') THEN COALESCE(gross_revenue_cents, 0)
        ELSE 0
      END
    ) AS gross_revenue_cents,

    SUM(
      CASE
        WHEN event_type IN ('order_paid', 'cash_collected') THEN COALESCE(net_revenue_cents, 0)
        ELSE 0
      END
    ) AS net_revenue_cents,

    SUM(
      CASE
        WHEN event_type = 'order_created' THEN COALESCE(discounts_cents, 0)
        ELSE 0
      END
    ) AS discounts_cents

  FROM %s
  WHERE %s
    AND occurred_at BETWEEN @start AND @end
  GROUP BY bucket_day
),
week_buckets AS (
  SELECT DATE_TRUNC(bucket_day, WEEK(MONDAY)) AS bucket_day
  FROM buckets
)
SELECT
  FORMAT_DATE('%%F', b.bucket_day) AS date, -- week-start date (YYYY-MM-DD)
  COALESCE(a.orders, 0) AS orders,
  COALESCE(a.gross_revenue_cents, 0) AS gross_revenue_cents,
  COALESCE(a.net_revenue_cents, 0) AS net_revenue_cents,
  COALESCE(a.discounts_cents, 0) AS discounts_cents
FROM week_buckets b
LEFT JOIN agg a USING(bucket_day)
ORDER BY b.bucket_day ASC
`

const bucketedMonthlySQL = `
WITH buckets AS (
  SELECT d AS bucket_day
  FROM UNNEST(
    GENERATE_DATE_ARRAY(
      DATE(@start, "` + centralTZ + `"),
      DATE(@end, "` + centralTZ + `"),
      INTERVAL 1 MONTH
    )
  ) AS d
),
agg AS (
  SELECT
    DATE_TRUNC(DATE(occurred_at, "` + centralTZ + `"), MONTH) AS bucket_day,

    COUNTIF(event_type = 'order_created') AS orders,

    SUM(
      CASE
        WHEN event_type IN ('order_paid', 'cash_collected') THEN COALESCE(gross_revenue_cents, 0)
        ELSE 0
      END
    ) AS gross_revenue_cents,

    SUM(
      CASE
        WHEN event_type IN ('order_paid', 'cash_collected') THEN COALESCE(net_revenue_cents, 0)
        ELSE 0
      END
    ) AS net_revenue_cents,

    SUM(
      CASE
        WHEN event_type = 'order_created' THEN COALESCE(discounts_cents, 0)
        ELSE 0
      END
    ) AS discounts_cents

  FROM %s
  WHERE %s
    AND occurred_at BETWEEN @start AND @end
  GROUP BY bucket_day
),
month_buckets AS (
  SELECT DATE_TRUNC(bucket_day, MONTH) AS bucket_day
  FROM buckets
)
SELECT
  FORMAT_DATE('%%Y-%%m', b.bucket_day) AS date, -- YYYY-MM
  COALESCE(a.orders, 0) AS orders,
  COALESCE(a.gross_revenue_cents, 0) AS gross_revenue_cents,
  COALESCE(a.net_revenue_cents, 0) AS net_revenue_cents,
  COALESCE(a.discounts_cents, 0) AS discounts_cents
FROM month_buckets b
LEFT JOIN agg a USING(bucket_day)
ORDER BY b.bucket_day ASC
`

const bucketedYearlySQL = `
WITH buckets AS (
  SELECT d AS bucket_day
  FROM UNNEST(
    GENERATE_DATE_ARRAY(
      DATE(@start, "` + centralTZ + `"),
      DATE(@end, "` + centralTZ + `"),
      INTERVAL 1 YEAR
    )
  ) AS d
),
agg AS (
  SELECT
    DATE_TRUNC(DATE(occurred_at, "` + centralTZ + `"), YEAR) AS bucket_day,

    COUNTIF(event_type = 'order_created') AS orders,

    SUM(
      CASE
        WHEN event_type IN ('order_paid', 'cash_collected') THEN COALESCE(gross_revenue_cents, 0)
        ELSE 0
      END
    ) AS gross_revenue_cents,

    SUM(
      CASE
        WHEN event_type IN ('order_paid', 'cash_collected') THEN COALESCE(net_revenue_cents, 0)
        ELSE 0
      END
    ) AS net_revenue_cents,

    SUM(
      CASE
        WHEN event_type = 'order_created' THEN COALESCE(discounts_cents, 0)
        ELSE 0
      END
    ) AS discounts_cents

  FROM %s
  WHERE %s
    AND occurred_at BETWEEN @start AND @end
  GROUP BY bucket_day
),
year_buckets AS (
  SELECT DATE_TRUNC(bucket_day, YEAR) AS bucket_day
  FROM buckets
)
SELECT
  FORMAT_DATE('%%Y', b.bucket_day) AS date, -- YYYY
  COALESCE(a.orders, 0) AS orders,
  COALESCE(a.gross_revenue_cents, 0) AS gross_revenue_cents,
  COALESCE(a.net_revenue_cents, 0) AS net_revenue_cents,
  COALESCE(a.discounts_cents, 0) AS discounts_cents
FROM year_buckets b
LEFT JOIN agg a USING(bucket_day)
ORDER BY b.bucket_day ASC
`
