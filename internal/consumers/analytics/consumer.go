package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cbigquery "cloud.google.com/go/bigquery"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/google/uuid"
)

const analyticsConsumerName = "analytics"

type tableInserter interface {
	InsertRows(ctx context.Context, table string, rows []any) error
}

type idempotencyChecker interface {
	CheckAndMarkProcessed(ctx context.Context, consumer string, eventID uuid.UUID) (bool, error)
	Delete(ctx context.Context, consumer string, eventID uuid.UUID) error
}

// Consumer writes marketplace events to BigQuery while honoring Redis idempotency.
type Consumer struct {
	client      tableInserter
	table       string
	manager     idempotencyChecker
	logg        *logger.Logger
	eventFilter map[enums.OutboxEventType]struct{}
}

// NewConsumer builds a new analytics consumer.
func NewConsumer(client tableInserter, table string, manager idempotencyChecker, logg *logger.Logger) (*Consumer, error) {
	if client == nil {
		return nil, fmt.Errorf("bigquery client required")
	}
	if strings.TrimSpace(table) == "" {
		return nil, fmt.Errorf("bigquery table name required")
	}
	if manager == nil {
		return nil, fmt.Errorf("idempotency manager required")
	}
	if logg == nil {
		return nil, fmt.Errorf("logger required")
	}
	return &Consumer{
		client:  client,
		table:   strings.TrimSpace(table),
		manager: manager,
		logg:    logg,
		eventFilter: map[enums.OutboxEventType]struct{}{
			enums.EventOrderCreated:  {},
			enums.EventCashCollected: {},
			enums.EventOrderPaid:     {},
		},
	}, nil
}

// Process ingests the outbox envelope into BigQuery if the event is supported.
func (c *Consumer) Process(ctx context.Context, eventType enums.OutboxEventType, envelope outbox.PayloadEnvelope) error {
	logCtx := c.logg.WithFields(ctx, map[string]any{
		"event_id":   envelope.EventID,
		"event_type": eventType,
	})

	if _, ok := c.eventFilter[eventType]; !ok {
		c.logg.Info(logCtx, "event not handled by analytics consumer")
		return nil
	}

	if envelope.EventID == "" {
		return fmt.Errorf("event id missing")
	}
	eventID, err := uuid.Parse(envelope.EventID)
	if err != nil {
		return fmt.Errorf("parse event id: %w", err)
	}

	already, err := c.manager.CheckAndMarkProcessed(ctx, analyticsConsumerName, eventID)
	if err != nil {
		return fmt.Errorf("idempotency check: %w", err)
	}
	if already {
		c.logg.Info(logCtx, "event already processed")
		return nil
	}

	row, err := buildRow(eventType, envelope)
	if err != nil {
		c.logg.Error(logCtx, "failed to build marketplace row", err)
		_ = c.manager.Delete(ctx, analyticsConsumerName, eventID)
		return err
	}

	if err := c.client.InsertRows(ctx, c.table, []any{row}); err != nil {
		c.logg.Error(logCtx, "failed to insert marketplace row", err)
		_ = c.manager.Delete(ctx, analyticsConsumerName, eventID)
		return err
	}

	c.logg.Info(logCtx, "marketplace event ingested")
	return nil
}

type marketplaceEventRow struct {
	EventID             string             `bigquery:"event_id"`
	EventType           string             `bigquery:"event_type"`
	OccurredAt          time.Time          `bigquery:"occurred_at"`
	CheckoutGroupID     *string            `bigquery:"checkout_group_id"`
	OrderID             *string            `bigquery:"order_id"`
	BuyerStoreID        *string            `bigquery:"buyer_store_id"`
	VendorStoreID       *string            `bigquery:"vendor_store_id"`
	AttributedAdClickID *string            `bigquery:"attributed_ad_click_id"`
	Payload             cbigquery.NullJSON `bigquery:"payload"`
}

func buildRow(eventType enums.OutboxEventType, envelope outbox.PayloadEnvelope) (*marketplaceEventRow, error) {
	payload := map[string]any{}
	if len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, &payload); err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
		if payload == nil {
			payload = map[string]any{}
		}
	}

	payloadJSON := cbigquery.NullJSON{}
	if len(envelope.Data) > 0 {
		payloadJSON.Valid = true
		payloadJSON.JSONVal = string(envelope.Data)
	}

	return &marketplaceEventRow{
		EventID:             envelope.EventID,
		EventType:           string(eventType),
		OccurredAt:          envelope.OccurredAt,
		CheckoutGroupID:     stringValue(payload, "checkout_group_id"),
		OrderID:             stringValue(payload, "order_id"),
		BuyerStoreID:        stringValue(payload, "buyer_store_id"),
		VendorStoreID:       stringValue(payload, "vendor_store_id"),
		AttributedAdClickID: stringValue(payload, "attributed_ad_click_id"),
		Payload:             payloadJSON,
	}, nil
}

func stringValue(payload map[string]any, key string) *string {
	if payload == nil {
		return nil
	}
	if raw, ok := payload[key]; ok {
		if str, ok := raw.(string); ok {
			trimmed := strings.TrimSpace(str)
			if trimmed != "" {
				return &trimmed
			}
		}
	}
	return nil
}
