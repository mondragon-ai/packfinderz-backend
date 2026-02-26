package reviews

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupReviewRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	const reviewsSchema = `
CREATE TABLE reviews (
  id TEXT PRIMARY KEY,
  review_type TEXT NOT NULL,
  buyer_store_id TEXT NOT NULL,
  buyer_user_id TEXT NOT NULL,
  vendor_store_id TEXT,
  product_id TEXT,
  order_id TEXT,
  rating INTEGER NOT NULL,
  title TEXT,
  body TEXT,
  is_verified_purchase INTEGER NOT NULL DEFAULT 0,
  is_visible INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME,
  updated_at DATETIME
);
`

	require.NoError(t, db.Exec(reviewsSchema).Error)
	return db
}

func TestRepository_CreateReview(t *testing.T) {
	db := setupReviewRepoTestDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	input := CreateReviewInput{
		ReviewType:         enums.ReviewTypeStore,
		BuyerStoreID:       uuid.New(),
		BuyerUserID:        uuid.New(),
		VendorStoreID:      ptrUUID(uuid.New()),
		Rating:             4,
		Title:              ptrString("Great service"),
		Body:               ptrString("Arrived on time"),
		IsVerifiedPurchase: true,
	}

	review, err := repo.CreateReview(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, review)
	assert.Equal(t, enums.ReviewTypeStore, review.ReviewType)
	assert.Equal(t, input.Rating, review.Rating)
	assert.True(t, review.IsVisible)
	assert.True(t, review.IsVerifiedPurchase)
	assert.Equal(t, *input.Title, *review.Title)
	assert.Equal(t, *input.Body, *review.Body)
}

func TestRepository_GetAndDeleteReview(t *testing.T) {
	db := setupReviewRepoTestDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	input := CreateReviewInput{
		ReviewType:   enums.ReviewTypeStore,
		BuyerStoreID: uuid.New(),
		BuyerUserID:  uuid.New(),
		Rating:       5,
	}

	review, err := repo.CreateReview(ctx, input)
	require.NoError(t, err)

	got, err := repo.GetReviewByID(ctx, review.ID)
	require.NoError(t, err)
	assert.Equal(t, review.ID, got.ID)

	require.NoError(t, repo.DeleteReview(ctx, review.ID))
	_, err = repo.GetReviewByID(ctx, review.ID)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestRepository_ListReviewsByVendorStoreID(t *testing.T) {
	db := setupReviewRepoTestDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	vendorID := uuid.New()
	buyerStoreID := uuid.New()
	buyerUserID := uuid.New()
	now := time.Now().UTC()

	// Insert visible reviews.
	insertReviewWithTime(t, repo, ctx, CreateReviewInput{
		ReviewType:   enums.ReviewTypeStore,
		BuyerStoreID: buyerStoreID,
		BuyerUserID:  buyerUserID,
		VendorStoreID: func() *uuid.UUID {
			id := vendorID
			return &id
		}(),
		Rating: 5,
	}, now.Add(-3*time.Hour), true)

	insertReviewWithTime(t, repo, ctx, CreateReviewInput{
		ReviewType:   enums.ReviewTypeStore,
		BuyerStoreID: buyerStoreID,
		BuyerUserID:  buyerUserID,
		VendorStoreID: func() *uuid.UUID {
			id := vendorID
			return &id
		}(),
		Rating: 4,
	}, now.Add(-2*time.Hour), true)

	// Insert a hidden review for the same vendor.
	insertReviewWithTime(t, repo, ctx, CreateReviewInput{
		ReviewType:   enums.ReviewTypeStore,
		BuyerStoreID: buyerStoreID,
		BuyerUserID:  buyerUserID,
		VendorStoreID: func() *uuid.UUID {
			id := vendorID
			return &id
		}(),
		Rating:    3,
		IsVisible: ptrBool(false),
	}, now.Add(-1*time.Hour), false)

	var visibleCount int64
	require.NoError(t, db.Model(&models.Review{}).
		Where("vendor_store_id = ? AND is_visible = 1", vendorID).
		Count(&visibleCount).Error)
	assert.Equal(t, int64(2), visibleCount)

	result, err := repo.ListReviewsByVendorStoreID(ctx, vendorID, true, "", 2)
	require.NoError(t, err)
	require.Len(t, result.Reviews, 2)
	assert.Equal(t, 2, result.Pagination.Total)
	assert.Empty(t, result.Pagination.Next)
	assert.NotEmpty(t, result.Pagination.First)
	assert.NotEmpty(t, result.Pagination.Last)
	assert.True(t, result.Reviews[0].CreatedAt.After(result.Reviews[1].CreatedAt))

	resultWithHidden, err := repo.ListReviewsByVendorStoreID(ctx, vendorID, false, "", 2)
	require.NoError(t, err)
	assert.Equal(t, 3, resultWithHidden.Pagination.Total)
	assert.NotEmpty(t, resultWithHidden.Pagination.Next)
}

func insertReviewWithTime(t *testing.T, repo *Repository, ctx context.Context, input CreateReviewInput, createdAt time.Time, isVisible bool) Review {
	t.Helper()

	var visibleVal *bool
	if isVisible {
		visibleVal = ptrBool(true)
	} else {
		visibleVal = ptrBool(false)
	}
	input.IsVisible = visibleVal

	review, err := repo.CreateReview(ctx, input)
	require.NoError(t, err)

	if !createdAt.IsZero() {
		visibleFlag := 0
		if isVisible {
			visibleFlag = 1
		}
		err = repo.db.WithContext(ctx).
			Model(&models.Review{}).
			Where("id = ?", review.ID).
			Updates(map[string]any{
				"created_at": createdAt,
				"updated_at": createdAt,
				"is_visible": visibleFlag,
			}).
			Error
		require.NoError(t, err)
		review.CreatedAt = createdAt
		review.UpdatedAt = createdAt
		review.IsVisible = isVisible
	}

	return *review
}

func ptrString(v string) *string {
	return &v
}

func ptrBool(v bool) *bool {
	return &v
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	return &id
}
