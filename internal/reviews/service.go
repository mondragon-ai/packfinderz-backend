package reviews

import (
	"context"
	"errors"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type reviewRepository interface {
	CreateReview(ctx context.Context, input CreateReviewInput) (*Review, error)
	ListReviewsByVendorStoreID(ctx context.Context, vendorStoreID uuid.UUID, visibleOnly bool, cursor string, limit int) (ReviewListResult, error)
}

type membershipRepository interface {
	GetMembership(ctx context.Context, userID, storeID uuid.UUID) (*models.StoreMembership, error)
}

type ordersRepository interface {
	HasBuyerStorePurchasedFromVendor(ctx context.Context, buyerStoreID, vendorStoreID uuid.UUID) (bool, error)
}

// Service exposes review business operations.
type Service interface {
	CreateReview(ctx context.Context, input CreateReviewInput) (*Review, error)
	ListVisibleReviews(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params) (ReviewListResult, error)
}

type service struct {
	repo        reviewRepository
	memberships membershipRepository
	orders      ordersRepository
}

// NewService builds a reviews service.
func NewService(repo reviewRepository, memberships membershipRepository, orders ordersRepository) Service {
	return &service{
		repo:        repo,
		memberships: memberships,
		orders:      orders,
	}
}

func (s *service) CreateReview(ctx context.Context, input CreateReviewInput) (*Review, error) {
	if input.ReviewType != enums.ReviewTypeStore {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "only store reviews are supported")
	}
	if input.BuyerStoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "buyer store id required")
	}
	if input.BuyerUserID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "buyer user id required")
	}
	if input.VendorStoreID == nil || *input.VendorStoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "vendor store id required")
	}

	membership, err := s.memberships.GetMembership(ctx, input.BuyerUserID, input.BuyerStoreID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.New(pkgerrors.CodeForbidden, "user is not a member of the buyer store")
		}
		return nil, err
	}
	if membership.Status != enums.MembershipStatusActive {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "membership is not active")
	}

	hasPurchase, err := s.orders.HasBuyerStorePurchasedFromVendor(ctx, input.BuyerStoreID, *input.VendorStoreID)
	if err != nil {
		return nil, err
	}
	if !hasPurchase {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "buyer store has no qualifying purchases with this vendor")
	}
	input.IsVerifiedPurchase = true

	return s.repo.CreateReview(ctx, input)
}

func (s *service) ListVisibleReviews(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params) (ReviewListResult, error) {
	if vendorStoreID == uuid.Nil {
		return ReviewListResult{}, pkgerrors.New(pkgerrors.CodeValidation, "vendor store id required")
	}
	return s.repo.ListReviewsByVendorStoreID(ctx, vendorStoreID, true, params.Cursor, params.Limit)
}
