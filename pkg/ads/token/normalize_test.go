package token

import (
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

func TestNormalizeTokensDeduplicatesAndOrders(t *testing.T) {
	t.Parallel()

	buyerID := uuid.New()
	vendorID := uuid.New()
	adID := uuid.New()
	now := time.Now().UTC()

	parser := testParser{
		parsed: map[string]Payload{
			"click-store-old": {
				TokenID:        uuid.New(),
				AdID:           adID,
				CreativeID:     uuid.New(),
				Placement:      enums.AdPlacementHero,
				TargetType:     enums.AdTargetTypeStore,
				TargetID:       vendorID,
				BuyerStoreID:   buyerID,
				EventType:      enums.AdEventFactTypeClick,
				OccurredAt:     now.Add(-2 * time.Minute),
				ExpiresAt:      now.Add(10 * time.Minute),
				RequestID:      "req-old",
				BidCents:       100,
				DestinationURL: "https://example.com",
			},
			"click-store-new": {
				TokenID:        uuid.New(),
				AdID:           adID,
				CreativeID:     uuid.New(),
				Placement:      enums.AdPlacementHero,
				TargetType:     enums.AdTargetTypeStore,
				TargetID:       vendorID,
				BuyerStoreID:   buyerID,
				EventType:      enums.AdEventFactTypeClick,
				OccurredAt:     now,
				ExpiresAt:      now.Add(15 * time.Minute),
				RequestID:      "req-new",
				BidCents:       200,
				DestinationURL: "https://example.com",
			},
			"impression-store": {
				TokenID:        uuid.New(),
				AdID:           adID,
				CreativeID:     uuid.New(),
				Placement:      enums.AdPlacementHero,
				TargetType:     enums.AdTargetTypeStore,
				TargetID:       vendorID,
				BuyerStoreID:   buyerID,
				EventType:      enums.AdEventFactTypeImpression,
				OccurredAt:     now.Add(5 * time.Minute),
				ExpiresAt:      now.Add(20 * time.Minute),
				RequestID:      "req-impr",
				BidCents:       50,
				DestinationURL: "https://example.com",
			},
			"click-product": {
				TokenID:        uuid.New(),
				AdID:           uuid.New(),
				CreativeID:     uuid.New(),
				Placement:      enums.AdPlacementProduct,
				TargetType:     enums.AdTargetTypeProduct,
				TargetID:       uuid.New(),
				BuyerStoreID:   buyerID,
				EventType:      enums.AdEventFactTypeClick,
				OccurredAt:     now.Add(-time.Minute),
				ExpiresAt:      now.Add(10 * time.Minute),
				RequestID:      "req-prod",
				BidCents:       150,
				DestinationURL: "https://example.com",
			},
		},
	}

	raw := []string{"click-store-old", "click-store-new", "impression-store", "click-product"}
	normalized := NormalizeTokens(raw, parser, buyerID)
	if len(normalized) != 3 {
		t.Fatalf("expected 3 tokens after dedupe, got %d", len(normalized))
	}
	if normalized[0].Raw != "click-store-new" {
		t.Fatalf("expected store click first, got %s", normalized[0].Raw)
	}
	if normalized[1].Raw != "click-product" {
		t.Fatalf("expected product click second, got %s", normalized[1].Raw)
	}
	if normalized[2].Raw != "impression-store" {
		t.Fatalf("expected impression last, got %s", normalized[2].Raw)
	}
}

func TestNormalizeTokensRespectsMaxBagSize(t *testing.T) {
	t.Parallel()

	buyerID := uuid.New()
	parser := testParser{parsed: map[string]Payload{}}
	raw := make([]string, 0, MaxAdTokenBagSize+5)
	for i := 0; i < MaxAdTokenBagSize+5; i++ {
		rawToken := uuid.NewString()
		tokenPayload := Payload{
			TokenID:        uuid.New(),
			AdID:           uuid.New(),
			CreativeID:     uuid.New(),
			Placement:      enums.AdPlacementStore,
			TargetType:     enums.AdTargetTypeStore,
			TargetID:       uuid.New(),
			BuyerStoreID:   buyerID,
			EventType:      enums.AdEventFactTypeClick,
			OccurredAt:     time.Now().Add(time.Duration(i) * time.Minute),
			ExpiresAt:      time.Now().Add(time.Duration(i+10) * time.Minute),
			RequestID:      "req-" + uuid.NewString(),
			BidCents:       100,
			DestinationURL: "https://example.com",
		}
		parser.parsed[rawToken] = tokenPayload
		raw = append(raw, rawToken)
	}

	normalized := NormalizeTokens(raw, parser, buyerID)
	if len(normalized) != MaxAdTokenBagSize {
		t.Fatalf("expected %d tokens, got %d", MaxAdTokenBagSize, len(normalized))
	}
	if normalized[0].Payload.OccurredAt.Before(normalized[len(normalized)-1].Payload.OccurredAt) {
		t.Fatalf("expected tokens ordered by recency")
	}
}

type testParser struct {
	parsed map[string]Payload
}

func (p testParser) Parse(value string) (Payload, error) {
	payload, ok := p.parsed[value]
	if !ok {
		return Payload{}, &parseError{token: value}
	}
	return payload, nil
}

type parseError struct {
	token string
}

func (e parseError) Error() string {
	return "unknown token " + e.token
}
