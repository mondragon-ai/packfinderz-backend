package checkout

import (
	"strings"

	"github.com/angelmondragon/packfinderz-backend/pkg/ads/token"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

func buildAttributionMaps(tokens []token.NormalizedToken) (map[uuid.UUID]*token.NormalizedToken, map[uuid.UUID]*token.NormalizedToken) {
	storeTokens := map[uuid.UUID]*token.NormalizedToken{}
	productTokens := map[uuid.UUID]*token.NormalizedToken{}
	for _, entry := range tokens {
		if entry.Payload.TargetID == uuid.Nil {
			continue
		}
		switch entry.Payload.TargetType {
		case enums.AdTargetTypeStore:
			storeTokens[entry.Payload.TargetID] = selectBetterToken(storeTokens[entry.Payload.TargetID], entry)
		case enums.AdTargetTypeProduct:
			productTokens[entry.Payload.TargetID] = selectBetterToken(productTokens[entry.Payload.TargetID], entry)
		}
	}
	return storeTokens, productTokens
}

func selectBetterToken(current *token.NormalizedToken, candidate token.NormalizedToken) *token.NormalizedToken {
	if current == nil {
		copy := candidate
		return &copy
	}
	if compareAttributionToken(candidate, *current) > 0 {
		copy := candidate
		return &copy
	}
	return current
}

func compareAttributionToken(a, b token.NormalizedToken) int {
	if pr := attributionPriority(a.Payload.EventType) - attributionPriority(b.Payload.EventType); pr != 0 {
		return pr
	}
	if !a.Payload.OccurredAt.Equal(b.Payload.OccurredAt) {
		if a.Payload.OccurredAt.After(b.Payload.OccurredAt) {
			return 1
		}
		return -1
	}
	return strings.Compare(a.Payload.TokenID.String(), b.Payload.TokenID.String())
}

func attributionPriority(event enums.AdEventFactType) int {
	switch event {
	case enums.AdEventFactTypeClick:
		return 2
	case enums.AdEventFactTypeImpression:
		return 1
	default:
		return 0
	}
}
