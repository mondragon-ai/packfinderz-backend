package types

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// Envelope represents the canonical analytics Pub/Sub envelope.
type Envelope struct {
	EventID       string                    `json:"event_id"`
	EventType     enums.AnalyticsEventType  `json:"event_type"`
	AggregateType enums.OutboxAggregateType `json:"aggregate_type"`
	AggregateID   string                    `json:"aggregate_id"`
	OccurredAt    time.Time                 `json:"occurred_at"`
	Payload       json.RawMessage           `json:"payload"`
}

// PayloadMap converts the raw payload to a map for keyed access.
func (e Envelope) PayloadMap() (map[string]any, error) {
	if len(bytes.TrimSpace(e.Payload)) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(e.Payload, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
