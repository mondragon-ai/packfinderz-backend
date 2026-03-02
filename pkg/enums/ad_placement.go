package enums

import "fmt"

// AdPlacement describes where an ad is intended to appear.
type AdPlacement string

const (
	AdPlacementHero    AdPlacement = "hero"
	AdPlacementStore   AdPlacement = "store"
	AdPlacementProduct AdPlacement = "product"
)

var validAdPlacements = []AdPlacement{
	AdPlacementHero,
	AdPlacementStore,
	AdPlacementProduct,
}

// String implements fmt.Stringer.
func (a AdPlacement) String() string {
	return string(a)
}

// IsValid reports whether the value is a known placement.
func (a AdPlacement) IsValid() bool {
	for _, candidate := range validAdPlacements {
		if candidate == a {
			return true
		}
	}
	return false
}

// ParseAdPlacement converts the raw string into an AdPlacement.
func ParseAdPlacement(value string) (AdPlacement, error) {
	for _, candidate := range validAdPlacements {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid ad placement %q", value)
}
