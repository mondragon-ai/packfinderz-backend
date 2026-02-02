package router

import (
	"fmt"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	analyticswriter "github.com/angelmondragon/packfinderz-backend/internal/analytics/writer"
)

func buildRevenueRow(envelope types.Envelope, amountCents int64, orderID, buyerStoreID, vendorStoreID string, occurred time.Time, payload any) (types.MarketplaceEventRow, error) {
	if occurred.IsZero() {
		occurred = envelope.OccurredAt
	}

	payloadJSON, err := analyticswriter.EncodeJSON(payload)
	if err != nil {
		return types.MarketplaceEventRow{}, fmt.Errorf("encode payload json: %w", err)
	}

	return types.MarketplaceEventRow{
		EventID:           envelope.EventID,
		EventType:         string(envelope.EventType),
		OccurredAt:        occurred.UTC(),
		OrderID:           stringPtr(orderID),
		BuyerStoreID:      stringPtr(buyerStoreID),
		VendorStoreID:     stringPtr(vendorStoreID),
		GrossRevenueCents: int64Ptr(amountCents),
		RefundCents:       int64Ptr(0),
		NetRevenueCents:   int64Ptr(amountCents),
		Payload:           payloadJSON,
	}, nil
}
