package enums

import "fmt"

// CartStatus tracks whether a cart record is active or already converted.
type CartStatus string

const (
	CartStatusActive    CartStatus = "active"
	CartStatusConverted CartStatus = "converted"
)

var validCartStatuses = []CartStatus{
	CartStatusActive,
	CartStatusConverted,
}

// String implements fmt.Stringer.
func (c CartStatus) String() string {
	return string(c)
}

// IsValid reports whether the value is a known CartStatus.
func (c CartStatus) IsValid() bool {
	for _, candidate := range validCartStatuses {
		if candidate == c {
			return true
		}
	}
	return false
}

// ParseCartStatus converts raw input into a CartStatus.
func ParseCartStatus(value string) (CartStatus, error) {
	for _, candidate := range validCartStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid cart status %q", value)
}
