package subscriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	subsvc "github.com/angelmondragon/packfinderz-backend/internal/subscriptions"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/google/uuid"
)

func TestVendorSubscriptionCreateRequiresVendor(t *testing.T) {
	handler := VendorSubscriptionCreate(&stubSubscriptionsService{}, logger.New(logger.Options{ServiceName: "test"}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vendor/subscriptions", bytes.NewReader([]byte(`{}`)))
	ctx := req.Context()
	ctx = middleware.WithStoreID(ctx, uuid.NewString())
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeBuyer)
	req = req.WithContext(ctx)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-vendor, got %d", resp.Code)
	}
}

func TestVendorSubscriptionCreateSuccess(t *testing.T) {
	sub := &models.Subscription{
		ID:                   uuid.New(),
		StripeSubscriptionID: "sub-123",
		Status:               enums.SubscriptionStatusActive,
		CurrentPeriodEnd:     time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
	}
	service := &stubSubscriptionsService{
		response: sub,
	}
	handler := VendorSubscriptionCreate(service, logger.New(logger.Options{ServiceName: "test"}))

	payload := vendorSubscriptionCreateRequest{
		StripeCustomerID:      "cust-1",
		StripePaymentMethodID: "pm-1",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vendor/subscriptions", bytes.NewReader(body))
	ctx := req.Context()
	ctx = middleware.WithStoreID(ctx, uuid.NewString())
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
	req = req.WithContext(ctx)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.Code)
	}
	if !service.calledCreate {
		t.Fatal("service should be invoked for create")
	}
	var envelope struct {
		Data vendorSubscriptionResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Data.StripeSubscriptionID != "sub-123" {
		t.Fatalf("unexpected payload")
	}
}

func TestVendorSubscriptionCancelRejectsNonVendor(t *testing.T) {
	handler := VendorSubscriptionCancel(&stubSubscriptionsService{}, logger.New(logger.Options{ServiceName: "test"}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vendor/subscriptions/cancel", nil)
	ctx := req.Context()
	ctx = middleware.WithStoreID(ctx, uuid.NewString())
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeBuyer)
	req = req.WithContext(ctx)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-vendor cancel, got %d", resp.Code)
	}
}

func TestVendorSubscriptionFetchReturnsNull(t *testing.T) {
	handler := VendorSubscriptionFetch(&stubSubscriptionsService{}, logger.New(logger.Options{ServiceName: "test"}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vendor/subscriptions", nil)
	ctx := req.Context()
	ctx = middleware.WithStoreID(ctx, uuid.NewString())
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
	req = req.WithContext(ctx)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if resp.Body.String() != `{"data":null}`+"\n" {
		t.Fatalf("expected null response, got %s", resp.Body.String())
	}
}

type stubSubscriptionsService struct {
	calledCreate bool
	response     *models.Subscription
	err          error
}

func (s *stubSubscriptionsService) Create(ctx context.Context, storeID uuid.UUID, input subsvc.CreateSubscriptionInput) (*models.Subscription, bool, error) {
	s.calledCreate = true
	return s.response, true, s.err
}

func (s *stubSubscriptionsService) Cancel(ctx context.Context, storeID uuid.UUID) error {
	return s.err
}

func (s *stubSubscriptionsService) GetActive(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	return nil, s.err
}
