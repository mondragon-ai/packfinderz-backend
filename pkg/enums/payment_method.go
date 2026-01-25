package enums

import "fmt"

// PaymentMethod describes how a buyer intends to settle an order.
type PaymentMethod string

const (
	PaymentMethodCash PaymentMethod = "cash"
	PaymentMethodACH  PaymentMethod = "ach"
)

var validPaymentMethods = []PaymentMethod{
	PaymentMethodCash,
	PaymentMethodACH,
}

// String implements fmt.Stringer.
func (p PaymentMethod) String() string {
	return string(p)
}

// IsValid reports whether the value is a known PaymentMethod.
func (p PaymentMethod) IsValid() bool {
	for _, candidate := range validPaymentMethods {
		if candidate == p {
			return true
		}
	}
	return false
}

// ParsePaymentMethod converts raw input into a PaymentMethod.
func ParsePaymentMethod(value string) (PaymentMethod, error) {
	for _, candidate := range validPaymentMethods {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid payment method %q", value)
}
