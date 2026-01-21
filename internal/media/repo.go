package media

import (
	"context"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
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
