package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

type testPaymentMethodsService struct {
	lastStoreID uuid.UUID
	result      []models.PaymentMethod
	err         error
}

func (s *testPaymentMethodsService) ListPaymentMethods(ctx context.Context, storeID uuid.UUID) ([]models.PaymentMethod, error) {
	s.lastStoreID = storeID
	return s.result, s.err
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
