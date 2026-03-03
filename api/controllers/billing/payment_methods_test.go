package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
)

type testPaymentMethodsService struct {
	lastStoreID     uuid.UUID
	result          []models.PaymentMethod
	err             error
	deleteErr       error
	deletedStoreID  uuid.UUID
	deletedMethodID uuid.UUID
}

func (s *testPaymentMethodsService) ListPaymentMethods(ctx context.Context, storeID uuid.UUID) ([]models.PaymentMethod, error) {
	s.lastStoreID = storeID
	return s.result, s.err
}

func (s *testPaymentMethodsService) DeletePaymentMethod(ctx context.Context, storeID, paymentMethodID uuid.UUID) error {
	s.deletedStoreID = storeID
	s.deletedMethodID = paymentMethodID
	return s.deleteErr
}

func TestVendorPaymentMethodsListRequiresVendorContext(t *testing.T) {
	handler := VendorPaymentMethodsList(&testPaymentMethodsService{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vendor/payment-methods", nil)
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without vendor context, got %d", resp.Code)
	}
}

func TestVendorPaymentMethodsListReturnsPaymentMethods(t *testing.T) {
	storeID := uuid.New()
	createdAt := time.Now().UTC()
	cardBrand := "visa"
	last4 := "4242"
	expMonth := 12
	expYear := 2025

	service := &testPaymentMethodsService{
		result: []models.PaymentMethod{
			{
				ID:        uuid.New(),
				CardBrand: &cardBrand,
				CardLast4: &last4,
				CardExpMonth: func() *int {
					v := expMonth
					return &v
				}(),
				CardExpYear: func() *int {
					v := expYear
					return &v
				}(),
				IsDefault: true,
				CreatedAt: createdAt,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vendor/payment-methods", nil)
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
	req = req.WithContext(ctx)

	handler := VendorPaymentMethodsList(service, nil)
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var envelope struct {
		Data vendorPaymentMethodsResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(envelope.Data.PaymentMethods) != 1 {
		t.Fatalf("expected 1 payment method, got %d", len(envelope.Data.PaymentMethods))
	}

	method := envelope.Data.PaymentMethods[0]
	if method.Brand == nil || *method.Brand != cardBrand {
		t.Fatalf("brand mismatch: got %v", method.Brand)
	}
	if method.Last4 == nil || *method.Last4 != last4 {
		t.Fatalf("last4 mismatch: got %v", method.Last4)
	}
	if method.ExpMonth == nil || *method.ExpMonth != expMonth {
		t.Fatalf("exp_month mismatch: got %v", method.ExpMonth)
	}
	if method.ExpYear == nil || *method.ExpYear != expYear {
		t.Fatalf("exp_year mismatch: got %v", method.ExpYear)
	}
	if !method.IsDefault {
		t.Fatalf("expected default true")
	}
	if !method.CreatedAt.Equal(createdAt) {
		t.Fatalf("created_at mismatch: got %v expected %v", method.CreatedAt, createdAt)
	}
	if service.lastStoreID != storeID {
		t.Fatalf("store id not passed through; expected %s got %s", storeID, service.lastStoreID)
	}
}

func TestVendorPaymentMethodDeleteReturnsNoContent(t *testing.T) {
	storeID := uuid.New()
	methodID := uuid.New()

	service := &testPaymentMethodsService{}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/vendor/payment-methods/"+methodID.String(), nil)
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
	req = req.WithContext(ctx)
	rc := chi.NewRouteContext()
	rc.URLParams.Add("paymentMethodId", methodID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

	handler := VendorPaymentMethodDelete(service, nil)
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.Code)
	}
	if service.deletedStoreID != storeID {
		t.Fatalf("expected store id %s, got %s", storeID, service.deletedStoreID)
	}
	if service.deletedMethodID != methodID {
		t.Fatalf("expected method id %s, got %s", methodID, service.deletedMethodID)
	}
}

func TestVendorPaymentMethodDeleteHandlesNotFound(t *testing.T) {
	storeID := uuid.New()
	methodID := uuid.New()

	service := &testPaymentMethodsService{
		deleteErr: pkgerrors.New(pkgerrors.CodeNotFound, "not found"),
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/vendor/payment-methods/"+methodID.String(), nil)
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
	req = req.WithContext(ctx)
	rc := chi.NewRouteContext()
	rc.URLParams.Add("paymentMethodId", methodID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

	handler := VendorPaymentMethodDelete(service, nil)
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}
