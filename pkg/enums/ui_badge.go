package enums

import "fmt"

// UIBadge represents optional badges shown in the billing UI.
type UIBadge string

const (
	UIBadgePopular   UIBadge = "popular"
	UIBadgeBestValue UIBadge = "best_value"
	UIBadgeNew       UIBadge = "new"
)

var validUIBadges = []UIBadge{
	UIBadgePopular,
	UIBadgeBestValue,
	UIBadgeNew,
}

// String implements fmt.Stringer.
func (u UIBadge) String() string {
	return string(u)
}

// IsValid reports whether the value is a known UIBadge.
func (u UIBadge) IsValid() bool {
	for _, candidate := range validUIBadges {
		if candidate == u {
			return true
		}
	}
	return false
}

// ParseUIBadge converts raw input into a UIBadge.
func ParseUIBadge(value string) (UIBadge, error) {
	for _, candidate := range validUIBadges {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid ui badge %q", value)
}
