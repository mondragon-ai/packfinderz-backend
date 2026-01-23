package notifications

import (
	"context"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"gorm.io/gorm"
)

// Repository manages notification persistence.
type Repository struct {
	db *gorm.DB
}

// NewRepository returns a notifications repository backed by the provided DB.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a notification row.
func (r *Repository) Create(ctx context.Context, notification *models.Notification) error {
	return r.db.WithContext(ctx).Create(notification).Error
}
