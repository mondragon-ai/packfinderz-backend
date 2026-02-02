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
}

type service struct {
	marketplace query.MarketplaceService
}

// NewService builds an analytics service backed by BigQuery.
func NewService(client *bigquery.Client, project, dataset, table string) (Service, error) {
	if client == nil {
		return nil, fmt.Errorf("bigquery client required")
	}

	marketplace, err := query.NewMarketplaceService(client, project, dataset, table)
	if err != nil {
		return nil, err
	}

	return &service{marketplace: marketplace}, nil
}

func (s *service) Query(ctx context.Context, req types.MarketplaceQueryRequest) (*types.MarketplaceQueryResponse, error) {
	return s.marketplace.Query(ctx, req)
}
