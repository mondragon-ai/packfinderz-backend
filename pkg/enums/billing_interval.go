package enums

import "fmt"

// BillingInterval defines the cadence for a billing plan.
type BillingInterval string

const (
	BillingIntervalEvery30Days BillingInterval = "EVERY_30_DAYS"
	BillingIntervalAnnual      BillingInterval = "ANNUAL"
)

var validBillingIntervals = []BillingInterval{
	BillingIntervalEvery30Days,
	BillingIntervalAnnual,
}

// String implements fmt.Stringer.
func (b BillingInterval) String() string {
	return string(b)
}

// IsValid reports whether the value is a known BillingInterval.
func (b BillingInterval) IsValid() bool {
	for _, candidate := range validBillingIntervals {
		if candidate == b {
			return true
		}
	}
	return false
}

// ParseBillingInterval converts raw input into a BillingInterval.
func ParseBillingInterval(value string) (BillingInterval, error) {
	for _, candidate := range validBillingIntervals {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid billing interval %q", value)
}
