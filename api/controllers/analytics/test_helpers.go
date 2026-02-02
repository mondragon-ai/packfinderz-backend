package analytics

import (
	"context"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
)

type testAnalyticsService struct {
	last     types.MarketplaceQueryRequest
	response *types.MarketplaceQueryResponse
	err      error
}

func (s *testAnalyticsService) Query(ctx context.Context, req types.MarketplaceQueryRequest) (*types.MarketplaceQueryResponse, error) {
	s.last = req
	if s.err != nil {
		return nil, s.err
	}
	if s.response == nil {
		s.response = &types.MarketplaceQueryResponse{}
	}
	return s.response, nil
}

func (s *testAnalyticsService) called() bool {
	return s.last.StoreID != ""
}

func (s *testAnalyticsService) period() time.Duration {
	return s.last.End.Sub(s.last.Start)
}
