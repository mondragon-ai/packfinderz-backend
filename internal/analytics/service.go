package analytics

import (
	"context"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/query"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/bigquery"
)

// Service provides analytics reports based on marketplace events.
type Service interface {
	// Query returns marketplace KPIs for the provided request.
	Query(ctx context.Context, req types.MarketplaceQueryRequest) (*types.MarketplaceQueryResponse, error)
	// QueryAd returns ad analytics for a store scoped to the provided ad ID.
	QueryAd(ctx context.Context, req types.AdQueryRequest) (*types.AdQueryResponse, error)
}

type service struct {
	marketplace query.MarketplaceService
	ad          query.AdService
}

// NewService builds an analytics service backed by BigQuery.
func NewService(client *bigquery.Client, project, dataset, marketplaceTable, adTable string) (Service, error) {
	if client == nil {
		return nil, fmt.Errorf("bigquery client required")
	}

	marketplace, err := query.NewMarketplaceService(client, project, dataset, marketplaceTable)
	if err != nil {
		return nil, err
	}

	ad, err := query.NewAdService(client, project, dataset, adTable, marketplaceTable)
	if err != nil {
		return nil, err
	}

	return &service{
		marketplace: marketplace,
		ad:          ad,
	}, nil
}

func (s *service) Query(ctx context.Context, req types.MarketplaceQueryRequest) (*types.MarketplaceQueryResponse, error) {
	return s.marketplace.Query(ctx, req)
}

func (s *service) QueryAd(ctx context.Context, req types.AdQueryRequest) (*types.AdQueryResponse, error) {
	return s.ad.Query(ctx, req)
}
