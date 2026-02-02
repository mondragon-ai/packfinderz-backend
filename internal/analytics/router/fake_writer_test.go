package router

import (
	"context"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
)

type fakeWriter struct {
	inserted []types.MarketplaceEventRow
}

func (f *fakeWriter) InsertMarketplace(_ context.Context, row types.MarketplaceEventRow) error {
	f.inserted = append(f.inserted, row)
	return nil
}

func (f *fakeWriter) InsertAdFact(_ context.Context, _ types.AdEventFactRow) error {
	return nil
}
