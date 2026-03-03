package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/internal/paymentmethods"
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
	updateStoreID   uuid.UUID
	updateMethodID  uuid.UUID
	updateIsDefault bool
	updateResult    *models.PaymentMethod
	updateErr       error
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

func (s *testPaymentMethodsService) UpdatePaymentMethodDefault(ctx context.Context, storeID, paymentMethodID uuid.UUID, isDefault bool) (*models.PaymentMethod, error) {
	s.updateStoreID = storeID
	s.updateMethodID = paymentMethodID
	s.updateIsDefault = isDefault
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	if s.updateResult != nil {
		return s.updateResult, nil
	}
	return &models.PaymentMethod{
		ID:        paymentMethodID,
		IsDefault: isDefault,
	}, nil
}

func (s *testPaymentMethodsService) StoreCard(ctx context.Context, storeID uuid.UUID, input paymentmethods.StoreCardInput) (*models.PaymentMethod, error) {
	return nil, nil
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

func TestVendorPaymentMethodUpdateRequiresVendorContext(t *testing.T) {
	handler := VendorPaymentMethodUpdate(&testPaymentMethodsService{}, nil)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/vendor/payment-methods/abc", strings.NewReader(`{"is_default": true}`))
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without vendor context, got %d", resp.Code)
	}
}

func TestVendorPaymentMethodUpdateRejectsMissingPayload(t *testing.T) {
	service := &testPaymentMethodsService{}

	storeID := uuid.New()
	methodID := uuid.New()

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/vendor/payment-methods/"+methodID.String(), strings.NewReader(`{}`))
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
	req = req.WithContext(ctx)
	rc := chi.NewRouteContext()
	rc.URLParams.Add("paymentMethodId", methodID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

	handler := VendorPaymentMethodUpdate(service, nil)
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when payload missing required field, got %d", resp.Code)
	}
}

func TestVendorPaymentMethodUpdateReturnsUpdatedMethod(t *testing.T) {
	storeID := uuid.New()
	methodID := uuid.New()
	cardBrand := "visa"
	updated := &models.PaymentMethod{
		ID:        methodID,
		CardBrand: &cardBrand,
		IsDefault: true,
		CreatedAt: time.Now().UTC(),
	}
	service := &testPaymentMethodsService{
		updateResult: updated,
	}

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/vendor/payment-methods/"+methodID.String(), strings.NewReader(`{"is_default": true}`))
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
	req = req.WithContext(ctx)
	rc := chi.NewRouteContext()
	rc.URLParams.Add("paymentMethodId", methodID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

	handler := VendorPaymentMethodUpdate(service, nil)
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var envelope struct {
		Data vendorPaymentMethodResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.ID != methodID.String() {
		t.Fatalf("expected id %s, got %s", methodID, envelope.Data.ID)
	}
	if envelope.Data.Brand == nil || *envelope.Data.Brand != cardBrand {
		t.Fatalf("expected brand %s, got %v", cardBrand, envelope.Data.Brand)
	}
	if !envelope.Data.IsDefault {
		t.Fatal("expected response to mark method as default")
	}
	if service.updateStoreID != storeID {
		t.Fatalf("expected store %s, got %s", storeID, service.updateStoreID)
	}
	if service.updateMethodID != methodID {
		t.Fatalf("expected method %s, got %s", methodID, service.updateMethodID)
	}
}

func TestVendorPaymentMethodUpdateHandlesNotFound(t *testing.T) {
	storeID := uuid.New()
	methodID := uuid.New()
	service := &testPaymentMethodsService{
		updateErr: pkgerrors.New(pkgerrors.CodeNotFound, "missing"),
	}

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/vendor/payment-methods/"+methodID.String(), strings.NewReader(`{"is_default": true}`))
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
	req = req.WithContext(ctx)
	rc := chi.NewRouteContext()
	rc.URLParams.Add("paymentMethodId", methodID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

	handler := VendorPaymentMethodUpdate(service, nil)
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}
