package outbox

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ActorRef identifies who produced the event.
type ActorRef struct {
	UserID  uuid.UUID  `json:"userId"`
	StoreID *uuid.UUID `json:"storeId,omitempty"`
	Role    string     `json:"role,omitempty"`
}

// PayloadEnvelope is the stable payload structure stored in outbox_events.
type PayloadEnvelope struct {
	Version    int             `json:"version"`
	EventID    string          `json:"eventId"`
	OccurredAt time.Time       `json:"occurredAt"`
	Actor      *ActorRef       `json:"actor,omitempty"`
	Data       json.RawMessage `json:"data"`
}
