package licenses

import (
	"context"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *Repository) DeleteWithTx(tx *gorm.DB, id uuid.UUID) error {
	if tx == nil {
		return gorm.ErrInvalidTransaction
	}
	return tx.Where("id = ?", id).Delete(&models.License{}).Error
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

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status enums.LicenseStatus) error {
	return r.db.WithContext(ctx).Model(&models.License{}).Where("id = ?", id).Update("status", status).Error
}

func (r *Repository) CreateWithTx(tx *gorm.DB, license *models.License) (*models.License, error) {
	if tx == nil {
		return nil, gorm.ErrInvalidTransaction
	}
	if err := tx.Create(license).Error; err != nil {
		return nil, err
	}
	return license, nil
}

func (r *Repository) FindByIDWithTx(tx *gorm.DB, id uuid.UUID) (*models.License, error) {
	if tx == nil {
		return nil, gorm.ErrInvalidTransaction
	}
	var row models.License
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repository) UpdateStatusWithTx(tx *gorm.DB, id uuid.UUID, status enums.LicenseStatus) error {
	if tx == nil {
		return gorm.ErrInvalidTransaction
	}
	return tx.Model(&models.License{}).Where("id = ?", id).Update("status", status).Error
}

func (r *Repository) FindExpiringBetween(ctx context.Context, from, to time.Time) ([]models.License, error) {
	var rows []models.License
	err := r.db.WithContext(ctx).
		Where("expiration_date >= ? AND expiration_date < ? AND status != ?", from, to, enums.LicenseStatusExpired).
		Find(&rows).Error
	return rows, err
}

func (r *Repository) FindExpiredByDate(ctx context.Context, day time.Time) ([]models.License, error) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	var rows []models.License
	err := r.db.WithContext(ctx).
		Where("expiration_date >= ? AND expiration_date < ? AND status != ?", start, end, enums.LicenseStatusExpired).
		Find(&rows).Error
	return rows, err
}

func (r *Repository) FindExpiredInRange(ctx context.Context, from, to time.Time) ([]models.License, error) {
	var rows []models.License
	err := r.db.WithContext(ctx).
		Where("expiration_date >= ? AND expiration_date < ? AND expiration_date IS NOT NULL AND status != ?", from, to, enums.LicenseStatusExpired).
		Find(&rows).Error
	return rows, err
}

func (r *Repository) FindExpiredBefore(ctx context.Context, cutoff time.Time) ([]models.License, error) {
	var rows []models.License
	err := r.db.WithContext(ctx).
		Where("expiration_date IS NOT NULL AND expiration_date <= ? AND status = ?", cutoff, enums.LicenseStatusExpired).
		Find(&rows).Error
	return rows, err
}

func (r *Repository) ListStatusesWithTx(tx *gorm.DB, storeID uuid.UUID) ([]enums.LicenseStatus, error) {
	if tx == nil {
		return nil, gorm.ErrInvalidTransaction
	}
	type statusRow struct {
		Status enums.LicenseStatus
	}
	var rows []statusRow
	if err := tx.Model(&models.License{}).Select("status").Where("store_id = ?", storeID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	statuses := make([]enums.LicenseStatus, 0, len(rows))
	for _, row := range rows {
		statuses = append(statuses, row.Status)
	}
	return statuses, nil
}
