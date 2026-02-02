package writer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	cbigquery "cloud.google.com/go/bigquery"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	pkgbigquery "github.com/angelmondragon/packfinderz-backend/pkg/bigquery"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultBatchSize      = 1
	defaultMaxAttempts    = 3
	defaultInitialBackoff = 250 * time.Millisecond
	defaultMaximumBackoff = 2 * time.Second
)

// Config controls the analytics writer behavior.
type Config struct {
	MarketplaceTable string
	AdEventTable     string
	BatchSize        int
	RetryPolicy      RetryPolicy
}

// RetryPolicy controls how many times BigQuery inserts are retried.
type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaximumBackoff time.Duration
}

type tableInserter interface {
	InsertRows(ctx context.Context, table string, rows []any) error
}

// BigQueryWriter inserts analytics rows into BigQuery with retries and optional batching.
type BigQueryWriter struct {
	client           tableInserter
	marketplaceTable string
	adEventTable     string
	batchSize        int
	retry            RetryPolicy

	marketplaceBuffer []types.MarketplaceEventRow
	adEventBuffer     []types.AdEventFactRow
}

// New creates a new BigQueryWriter backed by a shared client.
func New(client *pkgbigquery.Client, cfg Config) (*BigQueryWriter, error) {
	if client == nil {
		return nil, errors.New("bigquery client required")
	}
	mp := strings.TrimSpace(cfg.MarketplaceTable)
	if mp == "" {
		return nil, errors.New("marketplace table is required")
	}
	ad := strings.TrimSpace(cfg.AdEventTable)
	if ad == "" {
		return nil, errors.New("ad event table is required")
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	retry := cfg.RetryPolicy
	if retry.MaxAttempts <= 0 {
		retry.MaxAttempts = defaultMaxAttempts
	}
	if retry.InitialBackoff <= 0 {
		retry.InitialBackoff = defaultInitialBackoff
	}
	if retry.MaximumBackoff <= 0 {
		retry.MaximumBackoff = defaultMaximumBackoff
	}
	if retry.MaximumBackoff < retry.InitialBackoff {
		retry.MaximumBackoff = retry.InitialBackoff
	}

	return &BigQueryWriter{
		client:           client,
		marketplaceTable: mp,
		adEventTable:     ad,
		batchSize:        batchSize,
		retry:            retry,
	}, nil
}

// InsertMarketplace writes a single marketplace event row (flushes when batch size reached).
func (w *BigQueryWriter) InsertMarketplace(ctx context.Context, row types.MarketplaceEventRow) error {
	w.marketplaceBuffer = append(w.marketplaceBuffer, row)
	if len(w.marketplaceBuffer) >= w.batchSize {
		return w.flushMarketplace(ctx)
	}
	return nil
}

// InsertAdFact writes a single ad event fact row (flushes when batch size reached).
func (w *BigQueryWriter) InsertAdFact(ctx context.Context, row types.AdEventFactRow) error {
	w.adEventBuffer = append(w.adEventBuffer, row)
	if len(w.adEventBuffer) >= w.batchSize {
		return w.flushAdEvents(ctx)
	}
	return nil
}

// Flush writes any buffered rows immediately.
func (w *BigQueryWriter) Flush(ctx context.Context) error {
	if err := w.flushMarketplace(ctx); err != nil {
		return err
	}
	return w.flushAdEvents(ctx)
}

func (w *BigQueryWriter) flushMarketplace(ctx context.Context) error {
	if len(w.marketplaceBuffer) == 0 {
		return nil
	}
	rows := make([]any, len(w.marketplaceBuffer))
	for i := range w.marketplaceBuffer {
		rows[i] = &w.marketplaceBuffer[i]
	}

	if err := w.insertWithRetry(ctx, w.marketplaceTable, rows); err != nil {
		return err
	}
	w.marketplaceBuffer = w.marketplaceBuffer[:0]
	return nil
}

func (w *BigQueryWriter) flushAdEvents(ctx context.Context) error {
	if len(w.adEventBuffer) == 0 {
		return nil
	}
	rows := make([]any, len(w.adEventBuffer))
	for i := range w.adEventBuffer {
		rows[i] = &w.adEventBuffer[i]
	}

	if err := w.insertWithRetry(ctx, w.adEventTable, rows); err != nil {
		return err
	}
	w.adEventBuffer = w.adEventBuffer[:0]
	return nil
}

func (w *BigQueryWriter) insertWithRetry(ctx context.Context, table string, rows []any) error {
	if len(rows) == 0 {
		return nil
	}

	attempts := 0
	backoff := w.retry.InitialBackoff

	for {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
		}

		err := w.client.InsertRows(ctx, table, rows)
		if err == nil {
			return nil
		}

		attempts++
		if attempts >= w.retry.MaxAttempts || !isRetryableBigQueryError(err) {
			return fmt.Errorf("insert %s rows: %w", table, err)
		}

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		timer.Stop()

		backoff = minDuration(backoff*2, w.retry.MaximumBackoff)
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func isRetryableBigQueryError(err error) bool {
	if err == nil {
		return false
	}

	var multi *cbigquery.MultiError
	if errors.As(err, &multi) {
		if multi == nil || len(*multi) == 0 {
			return false
		}
		for _, inner := range *multi {
			if !isRetryableBigQueryError(inner) {
				return false
			}
		}
		return true
	}

	var pme *cbigquery.PutMultiError
	if errors.As(err, &pme) {
		if pme == nil || len(*pme) == 0 {
			return false
		}
		for _, rowErr := range *pme {
			if !isRetryableBigQueryError(rowErr.Errors) {
				return false
			}
		}
		return true
	}

	var rowErr *cbigquery.RowInsertionError
	if errors.As(err, &rowErr) {
		if rowErr == nil || len(rowErr.Errors) == 0 {
			return false
		}
		for _, inner := range rowErr.Errors {
			if !isRetryableBigQueryError(inner) {
				return false
			}
		}
		return true
	}

	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return isRetryableHTTPCode(apiErr.Code)
	}

	var statusErr interface{ GRPCStatus() *status.Status }
	if errors.As(err, &statusErr) {
		if st := statusErr.GRPCStatus(); st != nil {
			return isRetryableGRPCCode(st.Code())
		}
	}

	return false
}

func isRetryableHTTPCode(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusRequestTimeout,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isRetryableGRPCCode(code codes.Code) bool {
	switch code {
	case codes.Aborted,
		codes.DeadlineExceeded,
		codes.Internal,
		codes.ResourceExhausted,
		codes.Unavailable:
		return true
	default:
		return false
	}
}

// EncodeJSON serializes the provided payload so it can be stored in BigQuery JSON columns.
func EncodeJSON(payload any) (cbigquery.NullJSON, error) {
	switch value := payload.(type) {
	case nil:
		return cbigquery.NullJSON{}, nil
	case cbigquery.NullJSON:
		return value, nil
	case json.RawMessage:
		if len(value) == 0 {
			return cbigquery.NullJSON{}, nil
		}
		return cbigquery.NullJSON{Valid: true, JSONVal: string(value)}, nil
	case []byte:
		if len(value) == 0 {
			return cbigquery.NullJSON{}, nil
		}
		return cbigquery.NullJSON{Valid: true, JSONVal: string(value)}, nil
	}

	marshaled, err := json.Marshal(payload)
	if err != nil {
		return cbigquery.NullJSON{}, fmt.Errorf("marshal json: %w", err)
	}
	if len(marshaled) == 0 {
		return cbigquery.NullJSON{}, nil
	}
	return cbigquery.NullJSON{Valid: true, JSONVal: string(marshaled)}, nil
}
