package enums

import "fmt"

// AdEventFactType describes the allowed values for the `type` column in ad_event_facts.
type AdEventFactType string

const (
	AdEventFactTypeImpression AdEventFactType = "impression"
	AdEventFactTypeClick      AdEventFactType = "click"
	AdEventFactTypeConversion AdEventFactType = "conversion"
	AdEventFactTypeCharge     AdEventFactType = "charge"
)

var validAdEventFactTypes = []AdEventFactType{
	AdEventFactTypeImpression,
	AdEventFactTypeClick,
	AdEventFactTypeConversion,
	AdEventFactTypeCharge,
}

// IsValid reports whether the value matches the canonical ad event fact type enum.
func (a AdEventFactType) IsValid() bool {
	for _, candidate := range validAdEventFactTypes {
		if candidate == a {
			return true
		}
	}
	return false
}

// ParseAdEventFactType converts the raw string to AdEventFactType.
func ParseAdEventFactType(value string) (AdEventFactType, error) {
	for _, candidate := range validAdEventFactTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid ad event fact type %q", value)
}
