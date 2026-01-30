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
	return tx.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ? AND media_id = ?", entityType, entityID, mediaID).
		Delete(&models.MediaAttachment{}).
		Error
}
