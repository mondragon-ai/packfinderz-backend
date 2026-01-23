package licenses

import (
	"context"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
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

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*models.License, error) {
	var row models.License
	if err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.License{}).Error
}

func (r *Repository) CountValidLicenses(ctx context.Context, storeID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.License{}).
		Where("store_id = ? AND status = ?", storeID, enums.LicenseStatusVerified).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
