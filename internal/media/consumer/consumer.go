package consumer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	pubsub "cloud.google.com/go/pubsub/v2"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	objectFinalizeEvent  = "OBJECT_FINALIZE"
	payloadFormatJSONAPI = "JSON_API_V1"
)

type repository interface {
	FindByGCSKey(ctx context.Context, gcsKey string) (*models.Media, error)
	MarkUploaded(ctx context.Context, id uuid.UUID, uploadedAt time.Time) error
}

// Consumer processes GCS OBJECT_FINALIZE notifications from Pub/Sub.
type Consumer struct {
	repo         repository
	subscription *pubsub.Subscriber
	logg         *logger.Logger
	now          func() time.Time
}

// NewConsumer constructs a consumer that watches the provided subscription.
func NewConsumer(repo repository, subscription *pubsub.Subscriber, logg *logger.Logger) (*Consumer, error) {
	if repo == nil {
		return nil, errors.New("media repository is required")
	}
	if subscription == nil {
		return nil, errors.New("media subscription is required")
	}
	if logg == nil {
		return nil, errors.New("logger is required")
	}
	return &Consumer{
		repo:         repo,
		subscription: subscription,
		logg:         logg,
		now:          time.Now,
	}, nil
}

// Run processes messages until the context is canceled or the subscription errors.
func (c *Consumer) Run(ctx context.Context) error {
	return c.subscription.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		result := c.process(ctx, msg)
		if result.nack {
			msg.Nack()
			return
		}
		msg.Ack()
	})
}

type processResult struct {
	ack  bool
	nack bool
}

func (c *Consumer) process(ctx context.Context, msg *pubsub.Message) processResult {
	attrs := parseAttributes(msg.Attributes)
	fields := c.buildLogFields(msg.ID, attrs, nil)
	logCtx := c.logg.WithFields(ctx, fields)
	if attrs.EventType != objectFinalizeEvent {
		c.logg.Info(logCtx, "skipping non-finalize event")
		return processResult{ack: true}
	}
	if attrs.PayloadFormat != payloadFormatJSONAPI {
		c.logg.Warn(logCtx, "unsupported payload format")
		return processResult{ack: true}
	}

	payload, err := decodePayload(msg.Data)
	if err != nil {
		c.logg.Error(logCtx, "failed to decode payload", err)
		return processResult{ack: true}
	}

	var gcs gcsPayload
	if err := json.Unmarshal(payload, &gcs); err != nil {
		fields := c.buildLogFields(msg.ID, attrs, nil)
		fields["payload_preview"] = previewBytes(payload, 800) // small + safe
		fields["payload_len"] = len(payload)
		logCtx := c.logg.WithFields(ctx, fields)

		c.logg.Error(logCtx, "failed to unmarshal payload", err)
		return processResult{ack: true}
	}

	if strings.TrimSpace(gcs.Name) == "" {
		fields = c.buildLogFields(msg.ID, attrs, &gcs)
		logCtx = c.logg.WithFields(ctx, fields)
		c.logg.Error(logCtx, "payload missing gcs object name", fmt.Errorf("empty name"))
		return processResult{ack: true}
	}

	if attrs.ObjectID != "" && attrs.ObjectID != gcs.Name {
		fields = c.buildLogFields(msg.ID, attrs, &gcs)
		logCtx = c.logg.WithFields(ctx, fields)
		c.logg.Warn(logCtx, "attribute object_id differs from payload name")
	}

	fields = c.buildLogFields(msg.ID, attrs, &gcs)
	logCtx = c.logg.WithFields(ctx, fields)

	mediaRow, err := c.repo.FindByGCSKey(logCtx, gcs.Name)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.logg.Warn(logCtx, "media row not found")
			return processResult{ack: true}
		}
		return c.handleDBError(logCtx, err)
	}

	fields["media_id"] = mediaRow.ID.String()
	logCtx = c.logg.WithFields(ctx, fields)

	if isAlreadyProcessed(mediaRow.Status) {
		c.logg.Info(logCtx, "media status already handled")
		return processResult{ack: true}
	}

	uploadedAt := c.now()
	if err := c.repo.MarkUploaded(ctx, mediaRow.ID, uploadedAt); err != nil {
		return c.handleDBError(logCtx, err)
	}

	c.logg.Info(logCtx, "media marked as uploaded")
	return processResult{ack: true}
}

func (c *Consumer) handleDBError(ctx context.Context, err error) processResult {
	c.logg.Error(ctx, "media persistence error", err)
	if isTransientDBError(err) {
		return processResult{nack: true}
	}
	return processResult{ack: true}
}

func (c *Consumer) buildLogFields(messageID string, attrs gcsAttributes, payload *gcsPayload) map[string]any {
	fields := map[string]any{
		"message_id": messageID,
		"event_type": attrs.EventType,
		"bucket":     firstNonEmpty(attrs.BucketID, gcsBucket(payload)),
	}
	if payload != nil {
		fields["gcs_key"] = payload.Name
	}
	return fields
}

func gcsBucket(p *gcsPayload) string {
	if p == nil {
		return ""
	}
	return p.Bucket
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseAttributes(attrs map[string]string) gcsAttributes {
	return gcsAttributes{
		EventType:     attrs["eventType"],
		BucketID:      attrs["bucketId"],
		ObjectID:      attrs["objectId"],
		PayloadFormat: attrs["payloadFormat"],
	}
}

type gcsAttributes struct {
	EventType     string
	BucketID      string
	ObjectID      string
	PayloadFormat string
}

type gcsPayload struct {
	Name        string `json:"name"`
	Bucket      string `json:"bucket"`
	Generation  string `json:"generation"`
	ContentType string `json:"contentType"`
	Size        string `json:"size"`
}

func decodePayload(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("payload empty")
	}
	if decoded, err := base64.StdEncoding.DecodeString(string(data)); err == nil {
		return decoded, nil
	}
	return data, nil
}

func isAlreadyProcessed(status enums.MediaStatus) bool {
	switch status {
	case enums.MediaStatusUploaded, enums.MediaStatusProcessing, enums.MediaStatusReady:
		return true
	default:
		return false
	}
}

func isTransientDBError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

func previewBytes(b []byte, max int) string {
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "...(truncated)"
}
