package media

import (
	"context"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository exposes media metadata persistence operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository constructs a media repository bound to the provided GORM DB.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create persists a media record.
func (r *Repository) Create(ctx context.Context, media *models.Media) (*models.Media, error) {
	if err := r.db.WithContext(ctx).Create(media).Error; err != nil {
		return nil, err
	}
	return media, nil
}

// FindByID retrieves a media record by ID.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*models.Media, error) {
	var m models.Media
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Delete removes a media record.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Media{}).Error
}

// FindByGCSKey retrieves a media record using its GCS key.
func (r *Repository) FindByGCSKey(ctx context.Context, gcsKey string) (*models.Media, error) {
	var m models.Media
	if err := r.db.WithContext(ctx).First(&m, "gcs_key = ?", gcsKey).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// MarkUploaded marks the media row as uploaded and records the provided timestamp.
func (r *Repository) MarkUploaded(ctx context.Context, id uuid.UUID, uploadedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&models.Media{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":      enums.MediaStatusUploaded,
			"uploaded_at": uploadedAt,
		}).Error
}
