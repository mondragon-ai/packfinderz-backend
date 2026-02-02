package analytics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

type fakeMarketplaceService struct {
	lastReq  types.MarketplaceQueryRequest
	response *types.MarketplaceQueryResponse
	err      error
}

func (f *fakeMarketplaceService) Query(ctx context.Context, req types.MarketplaceQueryRequest) (*types.MarketplaceQueryResponse, error) {
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	if f.response == nil {
		f.response = &types.MarketplaceQueryResponse{}
	}
	return f.response, nil
}

func TestServiceQueryReturnsResponse(t *testing.T) {
	fake := &fakeMarketplaceService{}
	srv := &service{marketplace: fake}
	now := time.Now().UTC()
	req := types.MarketplaceQueryRequest{
		StoreID:   "store-id",
		StoreType: enums.StoreTypeVendor,
		Start:     now,
		End:       now.Add(2 * time.Hour),
	}

	resp, err := srv.Query(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != fake.response {
		t.Fatalf("expected response to be forwarded")
	}
	if fake.lastReq.StoreID != req.StoreID {
		t.Fatalf("unexpected request store id: %s", fake.lastReq.StoreID)
	}
	if fake.lastReq.StoreType != req.StoreType {
		t.Fatalf("unexpected store type: %s", fake.lastReq.StoreType)
	}
	if !fake.lastReq.Start.Equal(req.Start) || !fake.lastReq.End.Equal(req.End) {
		t.Fatalf("unexpected request window: %v - %v", fake.lastReq.Start, fake.lastReq.End)
	}
}

func TestServiceQueryPropagatesError(t *testing.T) {
	want := errors.New("query failed")
	fake := &fakeMarketplaceService{err: want}
	srv := &service{marketplace: fake}
	now := time.Now().UTC()
	req := types.MarketplaceQueryRequest{
		StoreID:   "store",
		StoreType: enums.StoreTypeVendor,
		Start:     now,
		End:       now.Add(time.Minute),
	}

	resp, err := srv.Query(context.Background(), req)
	if err != want {
		t.Fatalf("expected error %v, got %v", want, err)
	}
	if resp != nil {
		t.Fatalf("expected nil response on error")
	}
}
