package token

import (
	"sort"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

// MaxAdTokenBagSize limits how many tokens survive normalization.
const MaxAdTokenBagSize = 50

// NormalizedToken captures the parsed payload alongside the original string.
type NormalizedToken struct {
	Raw     string
	Payload Payload
}

type dedupeKey struct {
	EventType  enums.AdEventFactType
	TargetType enums.AdTargetType
	TargetID   uuid.UUID
	AdID       uuid.UUID
}

// NormalizeTokens validates, deduplicates, and caps the provided tokens.
func NormalizeTokens(raw []string, parser Parser, buyerStoreID uuid.UUID) []NormalizedToken {
	if parser == nil || len(raw) == 0 || buyerStoreID == uuid.Nil {
		return nil
	}

	normalized := make(map[dedupeKey]NormalizedToken, len(raw))
	for _, entry := range raw {
		payload, err := parser.Parse(entry)
		if err != nil {
			continue
		}
		if payload.BuyerStoreID != buyerStoreID {
			continue
		}
		key := dedupeKey{
			EventType:  payload.EventType,
			TargetType: payload.TargetType,
			TargetID:   payload.TargetID,
			AdID:       payload.AdID,
		}
		candidate := NormalizedToken{
			Raw:     entry,
			Payload: payload,
		}
		if current, ok := normalized[key]; !ok || compareByRecency(candidate.Payload, current.Payload) > 0 {
			normalized[key] = candidate
		}
	}

	if len(normalized) == 0 {
		return nil
	}

	tokens := make([]NormalizedToken, 0, len(normalized))
	for _, entry := range normalized {
		tokens = append(tokens, entry)
	}

	sort.Slice(tokens, func(i, j int) bool {
		return compareNormalized(tokens[i], tokens[j]) > 0
	})

	if len(tokens) > MaxAdTokenBagSize {
		tokens = tokens[:MaxAdTokenBagSize]
	}
	return tokens
}

func compareByRecency(a, b Payload) int {
	if !a.OccurredAt.Equal(b.OccurredAt) {
		if a.OccurredAt.After(b.OccurredAt) {
			return 1
		}
		return -1
	}
	return strings.Compare(a.TokenID.String(), b.TokenID.String())
}

func compareNormalized(a, b NormalizedToken) int {
	if pr := eventPriority(a.Payload.EventType) - eventPriority(b.Payload.EventType); pr != 0 {
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

func eventPriority(event enums.AdEventFactType) int {
	switch event {
	case enums.AdEventFactTypeClick:
		return 2
	case enums.AdEventFactTypeImpression:
		return 1
	default:
		return 0
	}
}
