package reviews

import (
	"context"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeReviewRepo struct {
	createInput  CreateReviewInput
	createReturn *Review
	createErr    error

	listArgs struct {
		vendorStoreID uuid.UUID
		visibleOnly   bool
		cursor        string
		limit         int
	}
	listReturn ReviewListResult
	listErr    error
}

func (f *fakeReviewRepo) CreateReview(ctx context.Context, input CreateReviewInput) (*Review, error) {
	f.createInput = input
	return f.createReturn, f.createErr
}

func (f *fakeReviewRepo) ListReviewsByVendorStoreID(ctx context.Context, vendorStoreID uuid.UUID, visibleOnly bool, cursor string, limit int) (ReviewListResult, error) {
	f.listArgs.vendorStoreID = vendorStoreID
	f.listArgs.visibleOnly = visibleOnly
	f.listArgs.cursor = cursor
	f.listArgs.limit = limit
	return f.listReturn, f.listErr
}

type fakeMembershipRepo struct {
	membership *models.StoreMembership
	err        error
}

func (f *fakeMembershipRepo) GetMembership(ctx context.Context, userID, storeID uuid.UUID) (*models.StoreMembership, error) {
	return f.membership, f.err
}

type fakeOrdersRepo struct {
	hasPurchase bool
	err         error
	calls       int
}

func (f *fakeOrdersRepo) HasBuyerStorePurchasedFromVendor(ctx context.Context, buyerStoreID, vendorStoreID uuid.UUID) (bool, error) {
	f.calls++
	return f.hasPurchase, f.err
}

func TestServiceCreateReviewSuccess(t *testing.T) {
	ctx := context.Background()
	repo := &fakeReviewRepo{
		createReturn: &Review{
			ID:           uuid.New(),
			ReviewType:   enums.ReviewTypeStore,
			BuyerStoreID: uuid.New(),
			Rating:       5,
		},
	}
	membershipRepo := &fakeMembershipRepo{
		membership: &models.StoreMembership{
			Status: enums.MembershipStatusActive,
		},
	}
	ordersRepo := &fakeOrdersRepo{hasPurchase: true}

	svc := NewService(repo, membershipRepo, ordersRepo)
	input := CreateReviewInput{
		ReviewType:   enums.ReviewTypeStore,
		BuyerStoreID: uuid.New(),
		BuyerUserID:  uuid.New(),
		VendorStoreID: func() *uuid.UUID {
			id := uuid.New()
			return &id
		}(),
		Rating: 5,
	}

	result, err := svc.CreateReview(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, repo.createReturn, result)
	assert.True(t, repo.createInput.IsVerifiedPurchase)
	assert.Equal(t, 1, ordersRepo.calls)
}

func TestServiceCreateReviewMembershipMissing(t *testing.T) {
	ctx := context.Background()
	repo := &fakeReviewRepo{}
	membershipRepo := &fakeMembershipRepo{
		err: gorm.ErrRecordNotFound,
	}
	ordersRepo := &fakeOrdersRepo{}

	svc := NewService(repo, membershipRepo, ordersRepo)
	_, err := svc.CreateReview(ctx, CreateReviewInput{
		ReviewType:   enums.ReviewTypeStore,
		BuyerStoreID: uuid.New(),
		BuyerUserID:  uuid.New(),
		VendorStoreID: func() *uuid.UUID {
			id := uuid.New()
			return &id
		}(),
		Rating: 4,
	})
	require.Error(t, err)
	typed := pkgerrors.As(err)
	require.NotNil(t, typed)
	assert.Equal(t, pkgerrors.CodeForbidden, typed.Code())
}

func TestServiceCreateReviewNoPurchase(t *testing.T) {
	ctx := context.Background()
	repo := &fakeReviewRepo{}
	membershipRepo := &fakeMembershipRepo{
		membership: &models.StoreMembership{
			Status: enums.MembershipStatusActive,
		},
	}
	ordersRepo := &fakeOrdersRepo{hasPurchase: false}

	svc := NewService(repo, membershipRepo, ordersRepo)
	_, err := svc.CreateReview(ctx, CreateReviewInput{
		ReviewType:   enums.ReviewTypeStore,
		BuyerStoreID: uuid.New(),
		BuyerUserID:  uuid.New(),
		VendorStoreID: func() *uuid.UUID {
			id := uuid.New()
			return &id
		}(),
		Rating: 4,
	})
	require.Error(t, err)
	typed := pkgerrors.As(err)
	require.NotNil(t, typed)
	assert.Equal(t, pkgerrors.CodeValidation, typed.Code())
	assert.Equal(t, 1, ordersRepo.calls)
}

func TestServiceListVisibleReviews(t *testing.T) {
	ctx := context.Background()
	repo := &fakeReviewRepo{
		listReturn: ReviewListResult{
			Reviews: []Review{
				{ID: uuid.New()},
			},
		},
	}
	membershipRepo := &fakeMembershipRepo{}
	ordersRepo := &fakeOrdersRepo{}
	svc := NewService(repo, membershipRepo, ordersRepo)

	vendorID := uuid.New()
	params := pagination.Params{
		Limit:  5,
		Cursor: "cursor",
	}
	result, err := svc.ListVisibleReviews(ctx, vendorID, params)
	require.NoError(t, err)
	assert.Equal(t, repo.listReturn, result)
	assert.Equal(t, vendorID, repo.listArgs.vendorStoreID)
	assert.True(t, repo.listArgs.visibleOnly)
	assert.Equal(t, params.Cursor, repo.listArgs.cursor)
	assert.Equal(t, params.Limit, repo.listArgs.limit)
}
