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

// List returns store-scoped licenses using cursor pagination.
func (r *Repository) List(ctx context.Context, opts listQuery) ([]models.License, error) {
	query := r.db.WithContext(ctx).Model(&models.License{}).Where("store_id = ?", opts.storeID)

	if opts.cursor != nil {
		query = query.Where("(created_at < ?) OR (created_at = ? AND id < ?)", opts.cursor.CreatedAt, opts.cursor.CreatedAt, opts.cursor.ID)
	}

	query = query.Order("created_at DESC").Order("id DESC").Limit(opts.limit)

	var rows []models.License
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
