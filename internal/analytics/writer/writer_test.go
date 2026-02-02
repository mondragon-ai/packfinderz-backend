package writer

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	pkgbigquery "github.com/angelmondragon/packfinderz-backend/pkg/bigquery"
	"google.golang.org/api/googleapi"
)

func TestNewWriterValidation(t *testing.T) {
	if _, err := New(nil, Config{}); err == nil {
		t.Fatal("expected error when client missing")
	}
	if _, err := New(&pkgbigquery.Client{}, Config{MarketplaceTable: " ", AdEventTable: "facts"}); err == nil {
		t.Fatal("expected error when marketplace table missing")
	}
	if _, err := New(&pkgbigquery.Client{}, Config{MarketplaceTable: "marketplace", AdEventTable: " "}); err == nil {
		t.Fatal("expected error when ad event table missing")
	}
}

func TestEncodeJSON(t *testing.T) {
	raw := map[string]any{"foo": "bar"}
	nj, err := EncodeJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error encoding json: %v", err)
	}
	if !nj.Valid {
		t.Fatal("expected json to be marked valid")
	}

	nj, err = EncodeJSON(nil)
	if err != nil {
		t.Fatalf("unexpected error for nil json: %v", err)
	}
	if nj.Valid {
		t.Fatal("expected nil json to be invalid")
	}

	rawMessage := json.RawMessage(`{"foo":"baz"}`)
	nj, err = EncodeJSON(rawMessage)
	if err != nil {
		t.Fatalf("unexpected error encoding raw json: %v", err)
	}
	if nj.JSONVal != string(rawMessage) {
		t.Fatalf("expected raw json passed through, got %s", nj.JSONVal)
	}
}

func TestWriterRetriesOnTransientError(t *testing.T) {
	writer, fake := newWriterWithFakeInserter(t)
	fake.responses = []error{
		&googleapi.Error{Code: http.StatusServiceUnavailable},
		nil,
	}

	if err := writer.InsertMarketplace(context.Background(), types.MarketplaceEventRow{EventID: "1"}); err != nil {
		t.Fatalf("unexpected error writing row: %v", err)
	}
	if len(fake.calls) != 2 {
		t.Fatalf("expected two insert attempts, got %d", len(fake.calls))
	}
	if fake.calls[1].table != writer.marketplaceTable {
		t.Fatalf("expected marketplace table on retry, got %s", fake.calls[1].table)
	}
	if len(writer.marketplaceBuffer) != 0 {
		t.Fatal("expected buffer to be empty after success")
	}
}

func TestWriterBatching(t *testing.T) {
	writer, fake := newWriterWithFakeInserter(t)
	writer.batchSize = 2

	if err := writer.InsertMarketplace(context.Background(), types.MarketplaceEventRow{EventID: "1"}); err != nil {
		t.Fatalf("unexpected error on first insert: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("expected no insert before batch full, got %d", len(fake.calls))
	}

	if err := writer.InsertMarketplace(context.Background(), types.MarketplaceEventRow{EventID: "2"}); err != nil {
		t.Fatalf("unexpected error on second insert: %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("expected single insert after batch flush, got %d", len(fake.calls))
	}
	if fake.calls[0].rowCount != 2 {
		t.Fatalf("expected two rows inserted, got %d", fake.calls[0].rowCount)
	}
}

func TestWriterFlush(t *testing.T) {
	writer, fake := newWriterWithFakeInserter(t)
	writer.batchSize = 10
	if err := writer.InsertMarketplace(context.Background(), types.MarketplaceEventRow{EventID: "1"}); err != nil {
		t.Fatalf("unexpected insert error: %v", err)
	}
	if err := writer.Flush(context.Background()); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("expected flush to insert once, got %d", len(fake.calls))
	}
	// if writer.marketplaceBuffer != nil && len(writer.marketplaceBuffer) != 0 {
	// 	t.Fatalf("expected buffer to be empty after flush, got %d", len(writer.marketplaceBuffer))
	// }
}

type insertCall struct {
	table    string
	rowCount int
}

type fakeInserter struct {
	responses []error
	calls     []insertCall
	index     int
}

func (f *fakeInserter) InsertRows(_ context.Context, table string, rows []any) error {
	f.calls = append(f.calls, insertCall{table: table, rowCount: len(rows)})
	var err error
	if f.index < len(f.responses) {
		err = f.responses[f.index]
	}
	f.index++
	return err
}

func newWriterWithFakeInserter(t *testing.T) (*BigQueryWriter, *fakeInserter) {
	t.Helper()
	writer, err := New(&pkgbigquery.Client{}, Config{
		MarketplaceTable: "marketplace_events",
		AdEventTable:     "ad_event_facts",
	})
	if err != nil {
		t.Fatalf("construct writer: %v", err)
	}

	fake := &fakeInserter{}
	writer.client = fake
	return writer, fake
}
