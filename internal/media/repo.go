package media

import (
	"context"
	"strings"
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

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

// List returns media rows matching the provided query.
func (r *Repository) List(ctx context.Context, opts listQuery) ([]models.Media, error) {
	query := r.db.WithContext(ctx).Model(&models.Media{}).Where("store_id = ?", opts.storeID)

	if opts.kind != nil {
		query = query.Where("kind = ?", *opts.kind)
	}
	if opts.status != nil {
		query = query.Where("status = ?", *opts.status)
	}
	if opts.mimeType != "" {
		query = query.Where("mime_type = ?", opts.mimeType)
	}
	if opts.search != "" {
		pattern := "%" + escapeLike(opts.search) + "%"
		query = query.Where("file_name ILIKE ? ESCAPE '\\'", pattern)
	}
	if opts.cursor != nil {
		query = query.Where("(created_at < ?) OR (created_at = ? AND id < ?)", opts.cursor.createdAt, opts.cursor.createdAt, opts.cursor.id)
	}

	query = query.Order("created_at DESC").Order("id DESC").Limit(opts.limit)

	var results []models.Media
	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
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
