package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	billingsvc "github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

type testBillingService struct {
	lastParams billingsvc.ListChargesParams
	result     *billingsvc.ListChargesResult
	err        error
}

func (s *testBillingService) ListCharges(ctx context.Context, params billingsvc.ListChargesParams) (*billingsvc.ListChargesResult, error) {
	s.lastParams = params
	return s.result, s.err
}

func TestVendorBillingChargesRequiresVendorContext(t *testing.T) {
	handler := VendorBillingCharges(&testBillingService{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vendor/billing/charges", nil)
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when vendor context missing, got %d", resp.Code)
	}
}

func TestVendorBillingChargesParsesFilters(t *testing.T) {
	storeID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vendor/billing/charges?limit=5&type=ad_spend&status=succeeded&cursor=test-cursor", nil)
	ctx := middleware.WithStoreID(req.Context(), storeID.String())
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
	req = req.WithContext(ctx)

	billedAt := time.Now().UTC()
	desc := "example"
	service := &testBillingService{
		result: &billingsvc.ListChargesResult{
			Items: []models.Charge{
				{
					ID:          uuid.New(),
					AmountCents: 1500,
					Currency:    "usd",
					Type:        enums.ChargeTypeAdSpend,
					Status:      enums.ChargeStatusSucceeded,
					Description: &desc,
					CreatedAt:   time.Now().UTC(),
					BilledAt:    &billedAt,
				},
			},
			Cursor: "next-page",
		},
	}

	handler := VendorBillingCharges(service, nil)
	resp := httptest.NewRecorder()
	handler(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var envelope struct {
		Data vendorBillingChargesResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	payload := envelope.Data
	if payload.Cursor != "next-page" {
		t.Fatalf("expected next cursor forwarded, got %q", payload.Cursor)
	}
	if len(payload.Charges) != 1 {
		t.Fatalf("expected 1 charge, got %d", len(payload.Charges))
	}

	charge := payload.Charges[0]
	if charge.AmountCents != 1500 {
		t.Fatalf("unexpected amount %d", charge.AmountCents)
	}
	if charge.Type != string(enums.ChargeTypeAdSpend) {
		t.Fatalf("unexpected type %s", charge.Type)
	}
	if charge.Status != string(enums.ChargeStatusSucceeded) {
		t.Fatalf("unexpected status %s", charge.Status)
	}
	if charge.Description == nil || *charge.Description != desc {
		t.Fatalf("description missing")
	}
	if charge.BilledAt == nil || !charge.BilledAt.Equal(billedAt) {
		t.Fatalf("unexpected billed_at")
	}

	if service.lastParams.StoreID != storeID {
		t.Fatalf("expected store id %s, got %s", storeID, service.lastParams.StoreID)
	}
	if service.lastParams.Limit != 5 {
		t.Fatalf("expected limit 5, got %d", service.lastParams.Limit)
	}
	if service.lastParams.Cursor != "test-cursor" {
		t.Fatalf("expected cursor preserved, got %s", service.lastParams.Cursor)
	}
	if service.lastParams.Type == nil || *service.lastParams.Type != enums.ChargeTypeAdSpend {
		t.Fatalf("type filter not forwarded")
	}
	if service.lastParams.Status == nil || *service.lastParams.Status != enums.ChargeStatusSucceeded {
		t.Fatalf("status filter not forwarded")
	}
}
