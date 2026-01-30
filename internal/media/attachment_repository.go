package media

import (
	"context"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// mediaAttachmentRepository wraps GORM operations for media_attachments.
type mediaAttachmentRepository struct {
	db *gorm.DB
}

// NewMediaAttachmentRepository builds a repository bound to the provided DB.
func NewMediaAttachmentRepository(db *gorm.DB) *mediaAttachmentRepository {
	return &mediaAttachmentRepository{db: db}
}

// Create inserts a media attachment row inside the provided transaction.
func (r *mediaAttachmentRepository) Create(ctx context.Context, tx *gorm.DB, attachment *models.MediaAttachment) error {
	return tx.WithContext(ctx).Create(attachment).Error
}

// Delete removes the attachment row matching the provided identifiers.
func (r *mediaAttachmentRepository) Delete(ctx context.Context, tx *gorm.DB, entityType string, entityID, mediaID uuid.UUID) error {
	executor := tx
	if executor == nil {
		executor = r.db
	}
	return executor.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ? AND media_id = ?", entityType, entityID, mediaID).
		Delete(&models.MediaAttachment{}).
		Error
}

// ListByMediaID returns attachments referencing the given media ID.
func (r *mediaAttachmentRepository) ListByMediaID(ctx context.Context, mediaID uuid.UUID) ([]models.MediaAttachment, error) {
	var attachments []models.MediaAttachment
	if err := r.db.WithContext(ctx).Where("media_id = ?", mediaID).Find(&attachments).Error; err != nil {
		return nil, err
	}
	return attachments, nil
}
