package media

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
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

func (r *Repository) DeleteWithTx(tx *gorm.DB, id uuid.UUID) error {
	if tx == nil {
		return gorm.ErrInvalidTransaction
	}
	return tx.Where("id = ?", id).Delete(&models.Media{}).Error
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

func applyMediaFilters(query *gorm.DB, opts listQuery) *gorm.DB {
	q := query.Where("store_id = ?", opts.storeID)
	if opts.kind != nil {
		q = q.Where("kind = ?", *opts.kind)
	}
	if opts.status != nil {
		q = q.Where("status = ?", *opts.status)
	}
	if opts.mimeType != "" {
		q = q.Where("mime_type = ?", opts.mimeType)
	}
	if opts.search != "" {
		pattern := "%" + escapeLike(opts.search) + "%"
		q = q.Where("file_name ILIKE ? ESCAPE '\\'", pattern)
	}
	return q
}

// ListPendingBefore returns pending media rows created before the cutoff.
func (r *Repository) ListPendingBefore(ctx context.Context, cutoff time.Time) ([]models.Media, error) {
	var results []models.Media
	if err := r.db.WithContext(ctx).
		Where("status = ? AND created_at < ?", enums.MediaStatusPending, cutoff).
		Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// List returns media rows matching the provided query.
func (r *Repository) List(ctx context.Context, opts listQuery) ([]models.Media, error) {
	query := applyMediaFilters(r.db.WithContext(ctx).Model(&models.Media{}), opts)
	if opts.cursor != nil {
		query = query.Where("(created_at < ?) OR (created_at = ? AND id < ?)", opts.cursor.CreatedAt, opts.cursor.CreatedAt, opts.cursor.ID)
	}
	query = query.Order("created_at DESC").Order("id DESC").Limit(opts.limit)

	var results []models.Media
	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// Count returns the total number of media rows matching the filters.
func (r *Repository) Count(ctx context.Context, opts listQuery) (int64, error) {
	var total int64
	query := applyMediaFilters(r.db.WithContext(ctx).Model(&models.Media{}), opts)
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// FetchBoundaryCursor returns the first/last cursor for the filtered media set.
func (r *Repository) FetchBoundaryCursor(ctx context.Context, opts listQuery, ascending bool) (string, error) {
	var row struct {
		CreatedAt time.Time
		ID        uuid.UUID
	}

	query := applyMediaFilters(r.db.WithContext(ctx).Model(&models.Media{}), opts).
		Select("created_at", "id")
	if ascending {
		query = query.Order("created_at ASC").Order("id ASC")
	} else {
		query = query.Order("created_at DESC").Order("id DESC")
	}
	query = query.Limit(1)

	if err := query.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return pagination.EncodeCursor(pagination.Cursor{CreatedAt: row.CreatedAt, ID: row.ID}), nil
}

// FindByGCSKey retrieves a media record using its GCS key.
func (r *Repository) FindByGCSKey(ctx context.Context, gcsKey string) (*models.Media, error) {
	var m models.Media
	if err := r.db.WithContext(ctx).First(&m, "gcs_key = ?", gcsKey).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// MarkUploaded marks the media row as uploaded, records the provided timestamp, and stores the public URL when available.
func (r *Repository) MarkUploaded(ctx context.Context, id uuid.UUID, uploadedAt time.Time, publicURL string) error {
	updates := map[string]any{
		"status":      enums.MediaStatusUploaded,
		"uploaded_at": uploadedAt,
	}
	if strings.TrimSpace(publicURL) != "" {
		updates["public_url"] = publicURL
	}
	return r.db.WithContext(ctx).Model(&models.Media{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// MarkDeleted marks the media as deleted with a timestamp.
func (r *Repository) MarkDeleted(ctx context.Context, id uuid.UUID, deletedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&models.Media{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     enums.MediaStatusDeleted,
			"deleted_at": deletedAt,
		}).Error
}
