package enums

import "fmt"

// CartItemWarningType enumerates warning reasons stored with cart items.
type CartItemWarningType string

const (
	CartItemWarningTypeClampedToMOQ   CartItemWarningType = "clamped_to_moq"
	CartItemWarningTypeClampedToMax   CartItemWarningType = "clamped_to_max"
	CartItemWarningTypePriceChanged   CartItemWarningType = "price_changed"
	CartItemWarningTypeNotAvailable   CartItemWarningType = "not_available"
	CartItemWarningTypeVendorInvalid  CartItemWarningType = "vendor_invalid"
	CartItemWarningTypeVendorMismatch CartItemWarningType = "vendor_mismatch"
	CartItemWarningTypeInvalidPromo   CartItemWarningType = "invalid_promo"
)

var validCartItemWarningTypes = []CartItemWarningType{
	CartItemWarningTypeClampedToMOQ,
	CartItemWarningTypeClampedToMax,
	CartItemWarningTypePriceChanged,
	CartItemWarningTypeNotAvailable,
	CartItemWarningTypeVendorInvalid,
	CartItemWarningTypeVendorMismatch,
	CartItemWarningTypeInvalidPromo,
}

// String implements fmt.Stringer.
func (c CartItemWarningType) String() string {
	return string(c)
}

// IsValid reports whether the value is known.
func (c CartItemWarningType) IsValid() bool {
	for _, candidate := range validCartItemWarningTypes {
		if candidate == c {
			return true
		}
	}
	return false
}

// ParseCartItemWarningType converts raw input into a CartItemWarningType.
func ParseCartItemWarningType(value string) (CartItemWarningType, error) {
	for _, candidate := range validCartItemWarningTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid cart item warning type %q", value)
}
