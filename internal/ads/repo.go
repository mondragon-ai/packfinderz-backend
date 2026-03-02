package ads

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository provides persistence operations for ads.
type Repository struct {
	db *gorm.DB
}

// NewRepository builds a repository tied to the provided DB.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// WithTx returns a repository bound to the provided transaction.
func (r *Repository) WithTx(tx *gorm.DB) *Repository {
	return &Repository{db: tx}
}

// CreateAd persists the input ad and returns the created row with creatives ordered deterministically.
func (r *Repository) CreateAd(ctx context.Context, input CreateAdInput) (*models.Ad, error) {
	if err := validateCreateAdInput(input); err != nil {
		return nil, err
	}

	ad := NewAdModelFromCreateInput(input)
	if err := r.db.WithContext(ctx).Create(&ad).Error; err != nil {
		return nil, err
	}

	var created models.Ad
	if err := r.db.WithContext(ctx).
		Preload("Creatives", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC").Order("id ASC")
		}).
		First(&created, "id = ?", ad.ID).
		Error; err != nil {
		return nil, err
	}

	return &created, nil
}

// ListAds returns a cursor-paginated list of ads scoped to the provided store.
func (r *Repository) ListAds(ctx context.Context, input ListAdsInput) (AdListResult, error) {
	if input.StoreID == uuid.Nil {
		return AdListResult{}, pkgerrors.New(pkgerrors.CodeValidation, "store_id is required")
	}
	if err := validateListFilters(input.Filters); err != nil {
		return AdListResult{}, err
	}

	page := input.Page
	if page <= 0 {
		page = 1
	}

	normalizedLimit := pagination.NormalizeLimit(input.Pagination.Limit)
	limitWithBuffer := pagination.LimitWithBuffer(input.Pagination.Limit)
	cursorValue := strings.TrimSpace(input.Pagination.Cursor)
	decodedCursor, err := pagination.ParseCursor(cursorValue)
	if err != nil {
		return AdListResult{}, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid cursor")
	}

	query := applyAdFilters(r.db.WithContext(ctx).Model(&models.Ad{}), input.StoreID, input.Filters)
	if decodedCursor != nil {
		query = query.Where("(created_at < ?) OR (created_at = ? AND id < ?)",
			decodedCursor.CreatedAt, decodedCursor.CreatedAt, decodedCursor.ID)
	}

	query = query.Preload("Creatives", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC").Order("id ASC")
	}).Order("created_at DESC").Order("id DESC").Limit(limitWithBuffer)

	var records []models.Ad
	if err := query.Find(&records).Error; err != nil {
		return AdListResult{}, err
	}

	resultRows := records
	nextCursor := ""
	if len(records) > normalizedLimit {
		resultRows = records[:normalizedLimit]
		last := resultRows[len(resultRows)-1]
		nextCursor = pagination.EncodeCursor(pagination.Cursor{
			CreatedAt: last.CreatedAt,
			ID:        last.ID,
		})
	}

	items := make([]AdDTO, len(resultRows))
	for i := range resultRows {
		items[i] = MapAdToDTO(&resultRows[i])
	}

	totalCount, err := r.countAds(ctx, input.StoreID, input.Filters)
	if err != nil {
		return AdListResult{}, err
	}
	firstCursor, err := r.fetchBoundaryCursor(ctx, input.StoreID, input.Filters, true)
	if err != nil {
		return AdListResult{}, err
	}
	lastCursor, err := r.fetchBoundaryCursor(ctx, input.StoreID, input.Filters, false)
	if err != nil {
		return AdListResult{}, err
	}

	prevCursor := ""
	if cursorValue != "" {
		prevCursor = cursorValue
	}

	paginationMeta := AdPagination{
		Page:    page,
		Total:   int(totalCount),
		Current: cursorValue,
		First:   firstCursor,
		Last:    lastCursor,
		Prev:    prevCursor,
		Next:    nextCursor,
	}

	return AdListResult{
		Ads:        items,
		Pagination: paginationMeta,
	}, nil
}

// GetAdByID fetches an ad by ID while enforcing the owning store.
func (r *Repository) GetAdByID(ctx context.Context, storeID uuid.UUID, adID uuid.UUID) (*models.Ad, error) {
	if storeID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "store_id is required")
	}
	if adID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "ad_id is required")
	}

	var ad models.Ad
	err := r.db.WithContext(ctx).
		Preload("Creatives", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC").Order("id ASC")
		}).
		First(&ad, "id = ? AND store_id = ?", adID, storeID).
		Error
	if err != nil {
		return nil, err
	}
	return &ad, nil
}

// ListEligibleAdsForServe returns active ads eligible for serving based on placement and schedule.
func (r *Repository) ListEligibleAdsForServe(ctx context.Context, placement enums.AdPlacement, limit int, now time.Time) ([]models.Ad, error) {
	if !placement.IsValid() {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "placement is required")
	}

	limit = pagination.NormalizeLimit(limit)
	query := r.db.WithContext(ctx).
		Model(&models.Ad{}).
		Preload("Creatives", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC").Order("id ASC")
		}).
		Where("status = ?", enums.AdStatusActive).
		Where("placement = ?", placement).
		Where("(starts_at IS NULL OR starts_at <= ?)", now).
		Where("(ends_at IS NULL OR ends_at >= ?)", now).
		Order("bid_cents DESC").
		Order("created_at DESC").
		Limit(limit)

	var ads []models.Ad
	if err := query.Find(&ads).Error; err != nil {
		return nil, err
	}
	return ads, nil
}

func validateCreateAdInput(input CreateAdInput) error {
	if input.StoreID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "store_id is required")
	}
	if !input.Status.IsValid() {
		return pkgerrors.New(pkgerrors.CodeValidation, "invalid status")
	}
	if !input.Placement.IsValid() {
		return pkgerrors.New(pkgerrors.CodeValidation, "invalid placement")
	}
	if !input.TargetType.IsValid() {
		return pkgerrors.New(pkgerrors.CodeValidation, "invalid target type")
	}
	if input.TargetID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "target_id is required")
	}
	if input.BidCents < 0 {
		return pkgerrors.New(pkgerrors.CodeValidation, "bid_cents must be non-negative")
	}
	if input.DailyBudgetCents < 0 {
		return pkgerrors.New(pkgerrors.CodeValidation, "daily_budget_cents must be non-negative")
	}
	if input.StartsAt != nil && input.EndsAt != nil && input.EndsAt.Before(*input.StartsAt) {
		return pkgerrors.New(pkgerrors.CodeValidation, "ends_at must be after starts_at")
	}
	if len(input.Creatives) == 0 {
		return pkgerrors.New(pkgerrors.CodeValidation, "at least one creative is required")
	}
	for idx, creative := range input.Creatives {
		if strings.TrimSpace(creative.DestinationURL) == "" {
			return pkgerrors.New(pkgerrors.CodeValidation, fmt.Sprintf("creative %d destination_url is required", idx+1))
		}
	}
	return nil
}

func validateListFilters(filters ListAdsFilters) error {
	if filters.Status != nil && !filters.Status.IsValid() {
		return pkgerrors.New(pkgerrors.CodeValidation, "invalid status filter")
	}
	if filters.Placement != nil && !filters.Placement.IsValid() {
		return pkgerrors.New(pkgerrors.CodeValidation, "invalid placement filter")
	}
	if filters.TargetType != nil && !filters.TargetType.IsValid() {
		return pkgerrors.New(pkgerrors.CodeValidation, "invalid target type filter")
	}
	return nil
}

func applyAdFilters(query *gorm.DB, storeID uuid.UUID, filters ListAdsFilters) *gorm.DB {
	q := query.Where("store_id = ?", storeID)
	if filters.Status != nil {
		q = q.Where("status = ?", *filters.Status)
	}
	if filters.Placement != nil {
		q = q.Where("placement = ?", *filters.Placement)
	}
	if filters.TargetType != nil {
		q = q.Where("target_type = ?", *filters.TargetType)
	}
	if filters.TargetID != nil && *filters.TargetID != uuid.Nil {
		q = q.Where("target_id = ?", *filters.TargetID)
	}
	return q
}

func (r *Repository) countAds(ctx context.Context, storeID uuid.UUID, filters ListAdsFilters) (int64, error) {
	var total int64
	query := applyAdFilters(r.db.WithContext(ctx).Model(&models.Ad{}), storeID, filters)
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *Repository) fetchBoundaryCursor(ctx context.Context, storeID uuid.UUID, filters ListAdsFilters, ascending bool) (string, error) {
	var row struct {
		CreatedAt time.Time
		ID        uuid.UUID
	}
	query := applyAdFilters(r.db.WithContext(ctx).Model(&models.Ad{}), storeID, filters).
		Select("created_at", "id")
	if ascending {
		query = query.Order("created_at ASC").Order("id ASC")
	} else {
		query = query.Order("created_at DESC").Order("id DESC")
	}
	query = query.Limit(1)

	if err := query.First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return pagination.EncodeCursor(pagination.Cursor{CreatedAt: row.CreatedAt, ID: row.ID}), nil
}
