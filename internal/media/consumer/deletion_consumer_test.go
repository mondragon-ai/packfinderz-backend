package consumer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	pubsub "cloud.google.com/go/pubsub/v2"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stubDeletionRepo struct {
	media       *models.Media
	attachments []models.MediaAttachment
	findErr     error
}

func (s *stubDeletionRepo) FindByGCSKey(ctx context.Context, gcsKey string) (*models.Media, error) {
	return s.media, s.findErr
}

func (s *stubDeletionRepo) ListByMediaID(ctx context.Context, mediaID uuid.UUID) ([]models.MediaAttachment, error) {
	return s.attachments, nil
}

type stubAttachmentRepo struct {
	attachments []models.MediaAttachment
	deleted     []models.MediaAttachment
	err         error
}

func (s *stubAttachmentRepo) Delete(ctx context.Context, tx *gorm.DB, entityType string, entityID, mediaID uuid.UUID) error {
	s.deleted = append(s.deleted, models.MediaAttachment{
		EntityType: entityType,
		EntityID:   entityID,
		MediaID:    mediaID,
	})
	return s.err
}

func (s *stubAttachmentRepo) ListByMediaID(ctx context.Context, mediaID uuid.UUID) ([]models.MediaAttachment, error) {
	return s.attachments, nil
}

type recordingDetacher struct {
	calls []models.MediaAttachment
	err   error
}

func (r *recordingDetacher) Detach(ctx context.Context, attachment models.MediaAttachment) error {
	r.calls = append(r.calls, attachment)
	return r.err
}

func encodePayload(payload gcsPayload) []byte {
	data, _ := json.Marshal(payload)
	return []byte(base64.StdEncoding.EncodeToString(data))
}

func buildMessage(name string) *pubsub.Message {
	return &pubsub.Message{
		Attributes: map[string]string{
			"eventType":     objectDeleteEvent,
			"payloadFormat": payloadFormatJSONAPI,
		},
		Data: encodePayload(gcsPayload{Name: name, Bucket: "packfinderz-media"}),
	}
}

func TestDeletionConsumerProcessesAttachments(t *testing.T) {
	t.Parallel()

	storeID := uuid.New()
	mediaID := uuid.New()
	attachments := []models.MediaAttachment{
		{ID: uuid.New(), EntityType: "store", EntityID: uuid.New(), MediaID: mediaID},
		{ID: uuid.New(), EntityType: "product", EntityID: uuid.New(), MediaID: mediaID},
	}

	repo := &stubDeletionRepo{
		media: &models.Media{
			ID:      mediaID,
			StoreID: storeID,
			GCSKey:  "packfinderz-media/path",
		},
		attachments: attachments,
	}
	attachmentRepo := &stubAttachmentRepo{attachments: attachments}
	detacher := &recordingDetacher{}
	sub := &pubsub.Subscriber{}
	logg := logger.New(logger.Options{ServiceName: "test"})
	consumer, err := NewDeletionConsumer(repo, attachmentRepo, detacher, sub, logg)
	if err != nil {
		t.Fatalf("NewDeletionConsumer: %v", err)
	}

	result := consumer.process(context.Background(), buildMessage(repo.media.GCSKey))
	if !result.ack || result.nack {
		t.Fatalf("expected ack result")
	}
	if len(detacher.calls) != len(attachments) {
		t.Fatalf("expected detacher called, got %d", len(detacher.calls))
	}
	if len(attachmentRepo.deleted) != len(attachments) {
		t.Fatalf("expected attachments deleted, got %d", len(attachmentRepo.deleted))
	}
}

func TestDeletionConsumerNacksOnDetacherError(t *testing.T) {
	t.Parallel()

	storeID := uuid.New()
	mediaID := uuid.New()

	repo := &stubDeletionRepo{
		media: &models.Media{
			ID:      mediaID,
			StoreID: storeID,
			GCSKey:  "media/object",
		},
		attachments: []models.MediaAttachment{
			{ID: uuid.New(), EntityType: "store", EntityID: uuid.New(), MediaID: mediaID},
		},
	}
	attachmentRepo := &stubAttachmentRepo{attachments: repo.attachments}
	detacher := &recordingDetacher{err: errors.New("boom")}
	sub := &pubsub.Subscriber{}
	logg := logger.New(logger.Options{ServiceName: "test"})
	consumer, _ := NewDeletionConsumer(repo, attachmentRepo, detacher, sub, logg)

	result := consumer.process(context.Background(), buildMessage(repo.media.GCSKey))
	if !result.nack {
		t.Fatalf("expected nack on detacher failure")
	}
	if len(attachmentRepo.deleted) != 0 {
		t.Fatalf("expected no deletes, got %d", len(attachmentRepo.deleted))
	}
}
