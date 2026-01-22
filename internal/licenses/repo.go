package licenses

import (
	"context"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"gorm.io/gorm"
)

// Repository exposes license persistence operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository constructs a license repository tied to the provided GORM DB.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new license row.
func (r *Repository) Create(ctx context.Context, license *models.License) (*models.License, error) {
	if err := r.db.WithContext(ctx).Create(license).Error; err != nil {
		return nil, err
	}
	return license, nil
}
