package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
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
		response: &types.MarketplaceQueryResponse{
			OrdersSeries: []types.TimeSeriesPoint{
				{Date: "2025-01-09", Value: 2},
			},
			GrossRevenue: []types.TimeSeriesPoint{
				{Date: "2025-01-09", Value: 400},
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
	if stub.last.StoreType != enums.StoreTypeVendor {
		t.Fatalf("expected vendor store type, got %s", stub.last.StoreType)
	}

	var respEnvelope struct {
		Data types.MarketplaceQueryResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respEnvelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(respEnvelope.Data.OrdersSeries) == 0 {
		t.Fatal("expected orders series data")
	}
	if respEnvelope.Data.OrdersSeries[0].Value != 2 {
		t.Fatalf("unexpected orders value: %d", respEnvelope.Data.OrdersSeries[0].Value)
	}
	if len(respEnvelope.Data.GrossRevenue) == 0 || respEnvelope.Data.GrossRevenue[0].Value != 400 {
		t.Fatalf("unexpected gross revenue data: %+v", respEnvelope.Data.GrossRevenue)
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
	if !stub.last.Start.Equal(startExpected) {
		t.Fatalf("unexpected start %v", stub.last.Start)
	}
	if !stub.last.End.Equal(endExpected) {
		t.Fatalf("unexpected end %v", stub.last.End)
	}
	if stub.last.StoreID != "vendor-1" {
		t.Fatalf("expected store id set, got %s", stub.last.StoreID)
	}
	if stub.last.StoreType != enums.StoreTypeVendor {
		t.Fatalf("expected vendor store type, got %s", stub.last.StoreType)
	}
}

type testAnalyticsService struct {
	last     types.MarketplaceQueryRequest
	response *types.MarketplaceQueryResponse
	err      error
}

func (s *testAnalyticsService) VendorAnalytics(ctx context.Context, req types.MarketplaceQueryRequest) (*types.MarketplaceQueryResponse, error) {
	s.last = req
	if s.response == nil {
		s.response = &types.MarketplaceQueryResponse{}
	}
	return s.response, s.err
}

func (s *testAnalyticsService) called() bool {
	return s.last.StoreID != ""
}

func (s *testAnalyticsService) period() time.Duration {
	return s.last.End.Sub(s.last.Start)
}
