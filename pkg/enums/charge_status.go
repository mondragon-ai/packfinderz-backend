package enums

import "fmt"

// ChargeStatus mirrors billing provider charge statuses.
type ChargeStatus string

const (
	ChargeStatusPending   ChargeStatus = "pending"
	ChargeStatusSucceeded ChargeStatus = "succeeded"
	ChargeStatusFailed    ChargeStatus = "failed"
	ChargeStatusRefunded  ChargeStatus = "refunded"
)

var validChargeStatuses = []ChargeStatus{
	ChargeStatusPending,
	ChargeStatusSucceeded,
	ChargeStatusFailed,
	ChargeStatusRefunded,
}

// String implements fmt.Stringer.
func (c ChargeStatus) String() string {
	return string(c)
}

// IsValid reports whether the value is known.
func (c ChargeStatus) IsValid() bool {
	for _, candidate := range validChargeStatuses {
		if candidate == c {
			return true
		}
	}
	return false
}

// ParseChargeStatus converts raw input into a ChargeStatus.
func ParseChargeStatus(value string) (ChargeStatus, error) {
	for _, candidate := range validChargeStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid charge status %q", value)
}
