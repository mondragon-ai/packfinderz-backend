package enums

import (
	"fmt"
	"strings"
)

// ChargeType categorizes billing charges.
type ChargeType string

const (
	ChargeTypeSubscription ChargeType = "subscription"
	ChargeTypeAdSpend      ChargeType = "ad_spend"
	ChargeTypeOther        ChargeType = "other"
)

var validChargeTypes = []ChargeType{
	ChargeTypeSubscription,
	ChargeTypeAdSpend,
	ChargeTypeOther,
}

// String implements fmt.Stringer.
func (c ChargeType) String() string {
	return string(c)
}

// IsValid reports whether the value is known.
func (c ChargeType) IsValid() bool {
	for _, candidate := range validChargeTypes {
		if candidate == c {
			return true
		}
	}
	return false
}

// ParseChargeType converts raw input into a ChargeType.
func ParseChargeType(value string) (ChargeType, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range validChargeTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid charge type %q", value)
}
