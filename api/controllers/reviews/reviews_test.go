package reviewcontrollers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/internal/reviews"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
)

type stubReviewsService struct {
	createFn func(ctx context.Context, input reviews.CreateReviewInput) (*reviews.Review, error)
	listFn   func(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params) (reviews.ReviewListResult, error)
	deleteFn func(ctx context.Context, reviewID, storeID, userID uuid.UUID) error
}

func (s *stubReviewsService) CreateReview(ctx context.Context, input reviews.CreateReviewInput) (*reviews.Review, error) {
	if s.createFn != nil {
		return s.createFn(ctx, input)
	}
	return nil, nil
}

func (s *stubReviewsService) ListVisibleReviews(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params) (reviews.ReviewListResult, error) {
	if s.listFn != nil {
		return s.listFn(ctx, vendorStoreID, params)
	}
	return reviews.ReviewListResult{}, nil
}

func (s *stubReviewsService) DeleteReview(ctx context.Context, reviewID, storeID, userID uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, reviewID, storeID, userID)
	}
	return nil
}

func TestCreateReviewController(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	vendorID := uuid.New()

	var received reviews.CreateReviewInput
	service := &stubReviewsService{
		createFn: func(ctx context.Context, input reviews.CreateReviewInput) (*reviews.Review, error) {
			received = input
			return &reviews.Review{
				ID:            uuid.New(),
				ReviewType:    enums.ReviewTypeStore,
				BuyerStoreID:  storeID,
				BuyerUserID:   userID,
				VendorStoreID: &vendorID,
				Rating:        5,
			}, nil
		},
	}

	payload := `{"vendor_store_id":"` + vendorID.String() + `","rating":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewBufferString(payload))
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithUserID(ctx, userID.String())
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := CreateReview(service, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rec.Code)
	}
	if received.BuyerStoreID != storeID {
		t.Fatalf("expected store id %s got %s", storeID, received.BuyerStoreID)
	}
	if received.BuyerUserID != userID {
		t.Fatalf("expected user id %s got %s", userID, received.BuyerUserID)
	}
	if received.VendorStoreID == nil || *received.VendorStoreID != vendorID {
		t.Fatalf("expected vendor id set")
	}
}

func TestListReviewsController(t *testing.T) {
	vendorID := uuid.New()
	service := &stubReviewsService{
		listFn: func(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params) (reviews.ReviewListResult, error) {
			return reviews.ReviewListResult{
				Reviews: []reviews.Review{
					{ID: uuid.New()},
				},
				Pagination: reviews.ReviewPagination{Page: 1, Total: 1},
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/stores/"+vendorID.String()+"/reviews?limit=5&cursor=test", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("storeId", vendorID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rec := httptest.NewRecorder()
	handler := ListReviews(service, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rec.Code)
	}
	var body struct {
		Data reviews.ReviewListResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data.Reviews) != 1 {
		t.Fatalf("expected 1 review")
	}
}

func TestDeleteReviewController(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	reviewID := uuid.New()
	var called bool
	service := &stubReviewsService{
		deleteFn: func(ctx context.Context, rid, sid, uid uuid.UUID) error {
			if rid != reviewID || sid != storeID || uid != userID {
				t.Fatalf("unexpected ids: %s %s %s", rid, sid, uid)
			}
			called = true
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/reviews/"+reviewID.String(), nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("reviewId", reviewID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	ctx = middleware.WithStoreID(ctx, storeID.String())
	ctx = middleware.WithUserID(ctx, userID.String())
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := DeleteReview(service, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rec.Code)
	}
	if !called {
		t.Fatalf("expected service delete called")
	}
}
