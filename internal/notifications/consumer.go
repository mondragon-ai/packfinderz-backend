package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	pubsub "cloud.google.com/go/pubsub/v2"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/idempotency"
	"github.com/google/uuid"
)

const licenseNotificationConsumer = "license-notifications"

type repository interface {
	Create(ctx context.Context, notification *models.Notification) error
}

// Consumer watches domain events and turns license status transitions into notifications.
type Consumer struct {
	repo         repository
	subscription *pubsub.Subscriber
	idempotency  *idempotency.Manager
	logg         *logger.Logger
}

// NewConsumer builds a license notification consumer.
func NewConsumer(repo repository, subscription *pubsub.Subscriber, manager *idempotency.Manager, logg *logger.Logger) (*Consumer, error) {
	if repo == nil {
		return nil, fmt.Errorf("notifications repository required")
	}
	if subscription == nil {
		return nil, fmt.Errorf("domain subscription required")
	}
	if manager == nil {
		return nil, fmt.Errorf("idempotency manager required")
	}
	if logg == nil {
		return nil, fmt.Errorf("logger required")
	}
	return &Consumer{
		repo:         repo,
		subscription: subscription,
		idempotency:  manager,
		logg:         logg,
	}, nil
}

// Run starts the consumer loop until the context is canceled.
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
	eventType := msg.Attributes["event_type"]
	fields := map[string]any{
		"message_id": msg.ID,
		"event_type": eventType,
	}
	logCtx := c.logg.WithFields(ctx, fields)

	if eventType != string(enums.EventLicenseStatusChanged) {
		c.logg.Info(logCtx, "skipping non-license event")
		return processResult{ack: true}
	}

	var envelope outbox.PayloadEnvelope
	if err := json.Unmarshal(msg.Data, &envelope); err != nil {
		c.logg.Error(logCtx, "failed to decode envelope", err)
		return processResult{ack: true}
	}

	eventID, err := uuid.Parse(envelope.EventID)
	if err != nil {
		c.logg.Error(logCtx, "invalid event id", err)
		return processResult{ack: true}
	}

	already, err := c.idempotency.CheckAndMarkProcessed(ctx, licenseNotificationConsumer, eventID)
	if err != nil {
		c.logg.Error(logCtx, "idempotency check failed", err)
		return processResult{nack: true}
	}
	if already {
		c.logg.Info(logCtx, "event already processed")
		return processResult{ack: true}
	}

	var payload licenseStatusChangedPayload
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		c.logg.Error(logCtx, "failed to parse payload", err)
		_ = c.idempotency.Delete(ctx, licenseNotificationConsumer, eventID)
		return processResult{nack: true}
	}

	fields["license_id"] = payload.LicenseID.String()
	logCtx = c.logg.WithFields(logCtx, map[string]any{
		"store_id": payload.StoreID.String(),
		"status":   payload.Status,
	})

	if err := c.handlePayload(ctx, payload, logCtx); err != nil {
		c.logg.Error(logCtx, "notification handling failed", err)
		_ = c.idempotency.Delete(ctx, licenseNotificationConsumer, eventID)
		return processResult{nack: true}
	}

	return processResult{ack: true}
}

func (c *Consumer) handlePayload(ctx context.Context, payload licenseStatusChangedPayload, logCtx context.Context) error {
	switch payload.Status {
	case enums.LicenseStatusPending:
		return c.createAdminNotification(ctx, payload, logCtx)
	case enums.LicenseStatusVerified, enums.LicenseStatusRejected:
		return c.createStoreNotification(ctx, payload, logCtx)
	default:
		c.logg.Info(logCtx, "status not handled")
		return nil
	}
}

func (c *Consumer) createStoreNotification(ctx context.Context, payload licenseStatusChangedPayload, logCtx context.Context) error {
	if payload.StoreID == uuid.Nil {
		return fmt.Errorf("store id missing")
	}
	licenseID := payload.LicenseID.String()
	link := fmt.Sprintf("/stores/%s/licenses/%s", payload.StoreID, payload.LicenseID)
	title := "License updated"
	message := fmt.Sprintf("License %s status changed to %s.", licenseID, payload.Status)
	if payload.Status == enums.LicenseStatusRejected && payload.Reason != "" {
		message = fmt.Sprintf("License %s was rejected. Reason: %s", licenseID, payload.Reason)
	}
	if payload.Status == enums.LicenseStatusVerified {
		title = "License approved"
		message = fmt.Sprintf("License %s has been verified.", licenseID)
	}
	notification := &models.Notification{
		StoreID: payload.StoreID,
		Type:    enums.NotificationTypeCompliance,
		Title:   title,
		Message: strings.TrimSpace(message),
		Link:    stringPtr(link),
	}
	if err := c.repo.Create(ctx, notification); err != nil {
		return err
	}
	c.logg.Info(logCtx, "store notified of license change")
	return nil
}

func (c *Consumer) createAdminNotification(ctx context.Context, payload licenseStatusChangedPayload, logCtx context.Context) error {
	if payload.StoreID == uuid.Nil {
		return fmt.Errorf("store id missing")
	}
	licenseID := payload.LicenseID.String()
	link := fmt.Sprintf("/admin/licenses/%s", payload.LicenseID)
	notification := &models.Notification{
		StoreID: payload.StoreID,
		Type:    enums.NotificationTypeCompliance,
		Title:   "License submitted for review",
		Message: fmt.Sprintf("Store %s submitted license %s for compliance review.", payload.StoreID, licenseID),
		Link:    stringPtr(link),
	}
	if err := c.repo.Create(ctx, notification); err != nil {
		return err
	}
	c.logg.Info(logCtx, "admin notified of pending license")
	return nil
}

func stringPtr(value string) *string {
	return &value
}

type licenseStatusChangedPayload struct {
	LicenseID uuid.UUID           `json:"licenseId"`
	StoreID   uuid.UUID           `json:"storeId"`
	Status    enums.LicenseStatus `json:"status"`
	Reason    string              `json:"reason,omitempty"`
}
