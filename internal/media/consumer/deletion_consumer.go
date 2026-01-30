package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	pubsub "cloud.google.com/go/pubsub/v2"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	objectDeleteEvent = "OBJECT_DELETE"
)

type deletionRepository interface {
	FindByGCSKey(ctx context.Context, gcsKey string) (*models.Media, error)
}

type attachmentRepository interface {
	Delete(ctx context.Context, tx *gorm.DB, entityType string, entityID, mediaID uuid.UUID) error
	ListByMediaID(ctx context.Context, mediaID uuid.UUID) ([]models.MediaAttachment, error)
}

type detachmentHandler interface {
	Detach(ctx context.Context, attachment models.MediaAttachment) error
}

// DeletionConsumer watches Pub/Sub for GCS OBJECT_DELETE notifications and removes attachments.
type DeletionConsumer struct {
	repo         deletionRepository
	attachments  attachmentRepository
	detacher     detachmentHandler
	subscription *pubsub.Subscriber
	logg         *logger.Logger
}

// NewDeletionConsumer wires the dependencies required for recursive media cleanup.
func NewDeletionConsumer(repo deletionRepository, attachments attachmentRepository, detacher detachmentHandler, subscription *pubsub.Subscriber, logg *logger.Logger) (*DeletionConsumer, error) {
	if repo == nil {
		return nil, errors.New("media repository is required")
	}
	if attachments == nil {
		return nil, errors.New("attachment repository is required")
	}
	if detacher == nil {
		return nil, errors.New("detacher is required")
	}
	if subscription == nil {
		return nil, errors.New("media deletion subscription is required")
	}
	if logg == nil {
		return nil, errors.New("logger is required")
	}
	return &DeletionConsumer{
		repo:         repo,
		attachments:  attachments,
		detacher:     detacher,
		subscription: subscription,
		logg:         logg,
	}, nil
}

// Run processes deletion notifications until the context is canceled.
func (c *DeletionConsumer) Run(ctx context.Context) error {
	return c.subscription.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		result := c.process(ctx, msg)
		if result.nack {
			msg.Nack()
			return
		}
		msg.Ack()
	})
}

func (c *DeletionConsumer) process(ctx context.Context, msg *pubsub.Message) processResult {
	attrs := parseAttributes(msg.Attributes)
	fields := c.buildLogFields(msg.ID, attrs, nil)
	logCtx := c.logg.WithFields(ctx, fields)

	if attrs.EventType != objectDeleteEvent {
		c.logg.Info(logCtx, "skipping non-delete event")
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
		fields = c.buildLogFields(msg.ID, attrs, nil)
		fields["payload_preview"] = previewBytes(payload, 800)
		fields["payload_len"] = len(payload)
		logCtx = c.logg.WithFields(ctx, fields)
		c.logg.Error(logCtx, "failed to unmarshal payload", err)
		return processResult{ack: true}
	}

	if gcs.Name == "" {
		fields = c.buildLogFields(msg.ID, attrs, &gcs)
		logCtx = c.logg.WithFields(ctx, fields)
		c.logg.Error(logCtx, "payload missing gcs object name", fmt.Errorf("empty name"))
		return processResult{ack: true}
	}

	fields = c.buildLogFields(msg.ID, attrs, &gcs)
	logCtx = c.logg.WithFields(ctx, fields)

	mediaRow, err := c.repo.FindByGCSKey(logCtx, gcs.Name)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.logg.Warn(logCtx, "media not found for deletion event")
			return processResult{ack: true}
		}
		return c.handleDBError(logCtx, err)
	}

	attachments, err := c.attachments.ListByMediaID(logCtx, mediaRow.ID)
	if err != nil {
		return c.handleDBError(logCtx, err)
	}

	sort.Slice(attachments, func(i, j int) bool {
		if attachments[i].EntityType != attachments[j].EntityType {
			return attachments[i].EntityType < attachments[j].EntityType
		}
		if attachments[i].EntityID != attachments[j].EntityID {
			return attachments[i].EntityID.String() < attachments[j].EntityID.String()
		}
		return attachments[i].ID.String() < attachments[j].ID.String()
	})

	for _, attachment := range attachments {
		if err := c.detacher.Detach(ctx, attachment); err != nil {
			c.logg.Error(logCtx, "domain detachment failed", err)
			return processResult{nack: true}
		}

		if err := c.attachments.Delete(ctx, nil, attachment.EntityType, attachment.EntityID, attachment.MediaID); err != nil {
			return c.handleDBError(logCtx, err)
		}
	}

	logCtx = c.logg.WithFields(logCtx, map[string]any{"attachments_processed": len(attachments)})
	c.logg.Info(logCtx, "processed media deletion event")
	return processResult{ack: true}
}

func (c *DeletionConsumer) handleDBError(ctx context.Context, err error) processResult {
	c.logg.Error(ctx, "media deletion db error", err)
	if isTransientDBError(err) {
		return processResult{nack: true}
	}
	return processResult{ack: true}
}

func (c *DeletionConsumer) buildLogFields(messageID string, attrs gcsAttributes, payload *gcsPayload) map[string]any {
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

type nopDetacher struct {
	logg *logger.Logger
}

func (n nopDetacher) Detach(ctx context.Context, attachment models.MediaAttachment) error {
	return nil
}

// NewNoopDetacher returns a detacher that only logs the attachment being removed.
func NewNoopDetacher(logg *logger.Logger) detachmentHandler {
	return nopDetacher{logg: logg}
}
