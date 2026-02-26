package reviews

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository encapsulates reviews persistence.
type Repository struct {
	db *gorm.DB
}

// NewRepository constructs a reviews repository bound to the provided gorm DB.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// CreateReview inserts a new review row and returns the created entity.
func (r *Repository) CreateReview(ctx context.Context, input CreateReviewInput) (*Review, error) {
	if !input.ReviewType.IsValid() {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "review type required")
	}
	if input.BuyerStoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "buyer store id required")
	}
	if input.BuyerUserID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "buyer user id required")
	}
	if input.Rating < 1 || input.Rating > 5 {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "rating must be between 1 and 5")
	}

	visible := true
	if input.IsVisible != nil {
		visible = *input.IsVisible
	}

	model := &models.Review{
		ReviewType:         input.ReviewType.String(),
		BuyerStoreID:       input.BuyerStoreID,
		BuyerUserID:        input.BuyerUserID,
		VendorStoreID:      input.VendorStoreID,
		ProductID:          input.ProductID,
		OrderID:            input.OrderID,
		Rating:             input.Rating,
		Title:              input.Title,
		Body:               input.Body,
		IsVerifiedPurchase: input.IsVerifiedPurchase,
		IsVisible:          visible,
	}
	if model.ID == uuid.Nil {
		model.ID = uuid.New()
	}

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return nil, err
	}

	review := reviewFromModel(model)
	return &review, nil
}

// GetReviewByID loads a review by its UUID.
func (r *Repository) GetReviewByID(ctx context.Context, reviewID uuid.UUID) (*Review, error) {
	if reviewID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "review id required")
	}

	var model models.Review
	if err := r.db.WithContext(ctx).
		Where("id = ?", reviewID).
		First(&model).Error; err != nil {
		return nil, err
	}
	review := reviewFromModel(&model)
	return &review, nil
}

// DeleteReview removes a review row.
func (r *Repository) DeleteReview(ctx context.Context, reviewID uuid.UUID) error {
	if reviewID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "review id required")
	}
	return r.db.WithContext(ctx).
		Where("id = ?", reviewID).
		Delete(&models.Review{}).
		Error
}

// ListReviewsByVendorStoreID returns a cursor page ordered by created_at DESC, id DESC.
func (r *Repository) ListReviewsByVendorStoreID(ctx context.Context, vendorStoreID uuid.UUID, visibleOnly bool, cursor string, limit int) (ReviewListResult, error) {
	if vendorStoreID == uuid.Nil {
		return ReviewListResult{}, pkgerrors.New(pkgerrors.CodeValidation, "vendor store id required")
	}

	normalizedLimit := pagination.NormalizeLimit(limit)
	limitWithBuffer := pagination.LimitWithBuffer(limit)
	cursorValue := strings.TrimSpace(cursor)
	decodedCursor, err := pagination.ParseCursor(cursorValue)
	if err != nil {
		return ReviewListResult{}, err
	}

	query := r.db.WithContext(ctx).
		Model(&models.Review{}).
		Where("vendor_store_id = ?", vendorStoreID)

	if visibleOnly {
		query = query.Where("is_visible = ?", true)
	}

	if decodedCursor != nil {
		query = query.Where("(created_at < ?) OR (created_at = ? AND id < ?)", decodedCursor.CreatedAt, decodedCursor.CreatedAt, decodedCursor.ID)
	}

	query = query.Order("created_at DESC").Order("id DESC").Limit(limitWithBuffer)

	var rows []models.Review
	if err := query.Find(&rows).Error; err != nil {
		return ReviewListResult{}, err
	}

	resultRows := rows
	nextCursor := ""
	if len(rows) > normalizedLimit {
		resultRows = rows[:normalizedLimit]
		last := resultRows[len(resultRows)-1]
		nextCursor = pagination.EncodeCursor(pagination.Cursor{
			CreatedAt: last.CreatedAt,
			ID:        last.ID,
		})
	}

	items := make([]Review, 0, len(resultRows))
	for _, row := range resultRows {
		items = append(items, reviewFromModel(&row))
	}

	count, err := r.countReviews(ctx, vendorStoreID, visibleOnly)
	if err != nil {
		return ReviewListResult{}, err
	}
	firstCursor, err := r.fetchReviewBoundaryCursor(ctx, vendorStoreID, visibleOnly, true)
	if err != nil {
		return ReviewListResult{}, err
	}
	lastCursor, err := r.fetchReviewBoundaryCursor(ctx, vendorStoreID, visibleOnly, false)
	if err != nil {
		return ReviewListResult{}, err
	}

	prevCursor := ""
	if cursorValue != "" {
		prevCursor = cursorValue
	}

	paginationMeta := ReviewPagination{
		Page:    1,
		Total:   int(count),
		Current: cursorValue,
		First:   firstCursor,
		Last:    lastCursor,
		Prev:    prevCursor,
		Next:    nextCursor,
	}

	return ReviewListResult{
		Reviews:    items,
		Pagination: paginationMeta,
	}, nil
}

func (r *Repository) countReviews(ctx context.Context, vendorStoreID uuid.UUID, visibleOnly bool) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).
		Model(&models.Review{}).
		Where("vendor_store_id = ?", vendorStoreID)
	if visibleOnly {
		query = query.Where("is_visible = ?", true)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) fetchReviewBoundaryCursor(ctx context.Context, vendorStoreID uuid.UUID, visibleOnly bool, ascending bool) (string, error) {
	order := "created_at DESC, id DESC"
	if ascending {
		order = "created_at ASC, id ASC"
	}

	query := r.db.WithContext(ctx).
		Model(&models.Review{}).
		Select("created_at", "id").
		Where("vendor_store_id = ?", vendorStoreID)
	if visibleOnly {
		query = query.Where("is_visible = ?", true)
	}
	query = query.Order(order).Limit(1)

	var row struct {
		CreatedAt time.Time
		ID        uuid.UUID
	}
	if err := query.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}

	return pagination.EncodeCursor(pagination.Cursor{
		CreatedAt: row.CreatedAt,
		ID:        row.ID,
	}), nil
}

func reviewFromModel(m *models.Review) Review {
	return Review{
		ID:                 m.ID,
		ReviewType:         enums.ReviewType(m.ReviewType),
		BuyerStoreID:       m.BuyerStoreID,
		BuyerUserID:        m.BuyerUserID,
		VendorStoreID:      m.VendorStoreID,
		ProductID:          m.ProductID,
		OrderID:            m.OrderID,
		Rating:             m.Rating,
		Title:              m.Title,
		Body:               m.Body,
		IsVerifiedPurchase: m.IsVerifiedPurchase,
		IsVisible:          m.IsVisible,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
}
