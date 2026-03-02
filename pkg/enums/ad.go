package enums

import "fmt"

// AdStatus tracks the lifecycle state of an ad.
type AdStatus string

const (
	AdStatusDraft     AdStatus = "draft"
	AdStatusActive    AdStatus = "active"
	AdStatusPaused    AdStatus = "paused"
	AdStatusExhausted AdStatus = "exhausted"
	AdStatusExpired   AdStatus = "expired"
	AdStatusArchived  AdStatus = "archived"
)

var validAdStatuses = []AdStatus{
	AdStatusDraft,
	AdStatusActive,
	AdStatusPaused,
	AdStatusExhausted,
	AdStatusExpired,
	AdStatusArchived,
}

// String implements fmt.Stringer.
func (a AdStatus) String() string {
	return string(a)
}

// IsValid reports whether the value is a known AdStatus.
func (a AdStatus) IsValid() bool {
	for _, candidate := range validAdStatuses {
		if candidate == a {
			return true
		}
	}
	return false
}

// ParseAdStatus converts text into an AdStatus.
func ParseAdStatus(value string) (AdStatus, error) {
	for _, candidate := range validAdStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid ad status %q", value)
}

// AdTargetType determines what the ad targets.
type AdTargetType string

const (
	AdTargetTypeStore   AdTargetType = "store"
	AdTargetTypeProduct AdTargetType = "product"
)

var validAdTargetTypes = []AdTargetType{
	AdTargetTypeStore,
	AdTargetTypeProduct,
}

// String implements fmt.Stringer.
func (a AdTargetType) String() string {
	return string(a)
}

// IsValid reports whether the value is a known AdTargetType.
func (a AdTargetType) IsValid() bool {
	for _, candidate := range validAdTargetTypes {
		if candidate == a {
			return true
		}
	}
	return false
}

// ParseAdTargetType converts text into an AdTargetType.
func ParseAdTargetType(value string) (AdTargetType, error) {
	for _, candidate := range validAdTargetTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid ad target type %q", value)
}
