package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

func TestVendorAnalyticsRequiresVendor(t *testing.T) {
	service := &testAnalyticsService{}
	handler := VendorAnalytics(service, logger.New(logger.Options{ServiceName: "test"}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vendor/analytics", nil)
	ctx := req.Context()
	ctx = middleware.WithStoreID(ctx, "store-1")
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeBuyer)
	req = req.WithContext(ctx)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-vendor, got %d", resp.Code)
	}
	if service.called() {
		t.Fatal("service should not be invoked for non-vendors")
	}
}

func TestVendorAnalyticsUsesPreset(t *testing.T) {
	now := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	timeNowUTC = func() time.Time { return now }
	defer func() { timeNowUTC = func() time.Time { return time.Now().UTC() } }()

	stub := &testAnalyticsService{
		response: &analytics.VendorAnalyticsResult{
			KPIs: analytics.VendorKPIs{
				Orders:             5,
				RevenueCents:       1000,
				AOVCents:           200,
				CashCollectedCents: 800,
			},
			Series: []analytics.VendorSeriesPoint{
				{Date: "2025-01-09", Orders: 2, RevenueCents: 400, CashCollectedCents: 300},
			},
		},
	}

	handler := VendorAnalytics(stub, logger.New(logger.Options{ServiceName: "test"}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vendor/analytics?preset=7d", nil)
	ctx := req.Context()
	ctx = middleware.WithStoreID(ctx, "store-1")
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
	req = req.WithContext(ctx)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.Code)
	}
	if stub.period() != 7*24*time.Hour {
		t.Fatalf("expected 7d range, got %v", stub.period())
	}

	var respEnvelope struct {
		Data analytics.VendorAnalyticsResult `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respEnvelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if respEnvelope.Data.KPIs.Orders != 5 {
		t.Fatalf("expected orders 5 got %d", respEnvelope.Data.KPIs.Orders)
	}
}

func TestVendorAnalyticsCustomRange(t *testing.T) {
	stub := &testAnalyticsService{}
	handler := VendorAnalytics(stub, logger.New(logger.Options{ServiceName: "test"}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vendor/analytics?from=2025-01-01T00:00:00Z&to=2025-01-07T00:00:00Z", nil)
	ctx := req.Context()
	ctx = middleware.WithStoreID(ctx, "vendor-1")
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeVendor)
	req = req.WithContext(ctx)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.Code)
	}
	startExpected := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endExpected := time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC)
	if !stub.last.start.Equal(startExpected) {
		t.Fatalf("unexpected start %v", stub.last.start)
	}
	if !stub.last.end.Equal(endExpected) {
		t.Fatalf("unexpected end %v", stub.last.end)
	}
}

type testAnalyticsService struct {
	last struct {
		vendorStoreID string
		start         time.Time
		end           time.Time
	}
	response *analytics.VendorAnalyticsResult
	err      error
}

func (s *testAnalyticsService) VendorAnalytics(ctx context.Context, vendorStoreID string, start, end time.Time) (*analytics.VendorAnalyticsResult, error) {
	s.last.vendorStoreID = vendorStoreID
	s.last.start = start
	s.last.end = end
	if s.response == nil {
		s.response = &analytics.VendorAnalyticsResult{}
	}
	return s.response, s.err
}

func (s *testAnalyticsService) called() bool {
	return s.last.vendorStoreID != ""
}

func (s *testAnalyticsService) period() time.Duration {
	return s.last.end.Sub(s.last.start)
}
