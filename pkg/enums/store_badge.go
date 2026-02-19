package enums

import "fmt"

// StoreBadge represents the store_badge enum in Postgres.
type StoreBadge string

const (
	StoreBadgeTopBrand        StoreBadge = "top_brand"
	StoreBadgeQualityVerified StoreBadge = "quality_verified"
)

var validStoreBadges = []StoreBadge{
	StoreBadgeTopBrand,
	StoreBadgeQualityVerified,
}

// String implements fmt.Stringer.
func (s StoreBadge) String() string {
	return string(s)
}

// IsValid reports whether the badge is a known value.
func (s StoreBadge) IsValid() bool {
	for _, candidate := range validStoreBadges {
		if candidate == s {
			return true
		}
	}
	return false
}

// ParseStoreBadge converts raw input into a StoreBadge.
func ParseStoreBadge(value string) (StoreBadge, error) {
	for _, candidate := range validStoreBadges {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid store badge %q", value)
}
