package enums

import "fmt"

// CartItemStatus tracks the validation state of a persisted cart line.
type CartItemStatus string

const (
	CartItemStatusOK           CartItemStatus = "ok"
	CartItemStatusNotAvailable CartItemStatus = "not_available"
	CartItemStatusInvalid      CartItemStatus = "invalid"
)

var validCartItemStatuses = []CartItemStatus{
	CartItemStatusOK,
	CartItemStatusNotAvailable,
	CartItemStatusInvalid,
}

// String implements fmt.Stringer.
func (c CartItemStatus) String() string {
	return string(c)
}

// IsValid reports whether the value is known.
func (c CartItemStatus) IsValid() bool {
	for _, candidate := range validCartItemStatuses {
		if candidate == c {
			return true
		}
	}
	return false
}

// ParseCartItemStatus converts raw input into a CartItemStatus.
func ParseCartItemStatus(value string) (CartItemStatus, error) {
	for _, candidate := range validCartItemStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid cart item status %q", value)
}
