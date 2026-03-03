package token

import (
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMintAndParseTokenRoundTrip(t *testing.T) {
	payload := Payload{
		TokenID:        uuid.New(),
		AdID:           uuid.New(),
		CreativeID:     uuid.New(),
		Placement:      enums.AdPlacementHero,
		TargetType:     enums.AdTargetTypeStore,
		TargetID:       uuid.New(),
		BuyerStoreID:   uuid.New(),
		EventType:      enums.AdEventFactTypeImpression,
		OccurredAt:     time.Now().UTC(),
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
		RequestID:      "req-123",
		BidCents:       150,
		DestinationURL: "https://example.com",
	}

	tokenString, err := MintToken("secret", payload)
	require.NoError(t, err)

	parsed, err := ParseToken("secret", tokenString)
	require.NoError(t, err)
	require.Equal(t, payload.AdID, parsed.AdID)
	require.Equal(t, payload.CreativeID, parsed.CreativeID)
	require.Equal(t, payload.Placement, parsed.Placement)
	require.Equal(t, payload.TargetType, parsed.TargetType)
	require.Equal(t, payload.TargetID, parsed.TargetID)
	require.Equal(t, payload.BuyerStoreID, parsed.BuyerStoreID)
	require.Equal(t, payload.EventType, parsed.EventType)
	require.Equal(t, payload.RequestID, parsed.RequestID)
	require.Equal(t, payload.BidCents, parsed.BidCents)
	require.Equal(t, payload.DestinationURL, parsed.DestinationURL)
}
