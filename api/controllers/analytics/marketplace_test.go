package analytics

import (
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

func TestMarketplaceAnalyticsRequiresStoreContext(t *testing.T) {
	stub := &testAnalyticsService{}
	handler := MarketplaceAnalytics(stub, logger.New(logger.Options{ServiceName: "test"}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/marketplace", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when store context missing, got %d", resp.Code)
	}
	if stub.called() {
		t.Fatal("service should not be invoked when context missing")
	}
}

func TestMarketplaceAnalyticsUsesPreset(t *testing.T) {
	now := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	timeNowUTC = func() time.Time { return now }
	defer func() { timeNowUTC = func() time.Time { return time.Now().UTC() } }()

	stub := &testAnalyticsService{
		response: &types.MarketplaceQueryResponse{
			OrdersSeries: []types.TimeSeriesPoint{
				{Date: "2025-01-09", Value: 3},
			},
			GrossRevenue: []types.TimeSeriesPoint{
				{Date: "2025-01-09", Value: 500},
			},
		},
	}

	handler := MarketplaceAnalytics(stub, logger.New(logger.Options{ServiceName: "test"}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/marketplace?preset=7d", nil)
	ctx := req.Context()
	ctx = middleware.WithStoreID(ctx, "store-1")
	ctx = middleware.WithStoreType(ctx, enums.StoreTypeBuyer)
	req = req.WithContext(ctx)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.Code)
	}
	if stub.period() != 7*24*time.Hour {
		t.Fatalf("expected 7d range, got %v", stub.period())
	}
	if stub.last.StoreType != enums.StoreTypeBuyer {
		t.Fatalf("expected buyer store type, got %s", stub.last.StoreType)
	}

	var envelope struct {
		Data types.MarketplaceQueryResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data.OrdersSeries) == 0 || envelope.Data.OrdersSeries[0].Value != 3 {
		t.Fatalf("unexpected orders blob: %+v", envelope.Data.OrdersSeries)
	}
	if len(envelope.Data.GrossRevenue) == 0 || envelope.Data.GrossRevenue[0].Value != 500 {
		t.Fatalf("unexpected revenue blob: %+v", envelope.Data.GrossRevenue)
	}
}
