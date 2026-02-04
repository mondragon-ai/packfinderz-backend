package enums

import "fmt"

// PaymentMethodType mirrors the billing provider's payment method categories.
type PaymentMethodType string

const (
	PaymentMethodTypeCard          PaymentMethodType = "card"
	PaymentMethodTypeUSBankAccount PaymentMethodType = "us_bank_account"
	PaymentMethodTypeOther         PaymentMethodType = "other"
)

var validPaymentMethodTypes = []PaymentMethodType{
	PaymentMethodTypeCard,
	PaymentMethodTypeUSBankAccount,
	PaymentMethodTypeOther,
}

// String implements fmt.Stringer.
func (p PaymentMethodType) String() string {
	return string(p)
}

// IsValid reports whether the value is known.
func (p PaymentMethodType) IsValid() bool {
	for _, candidate := range validPaymentMethodTypes {
		if candidate == p {
			return true
		}
	}
	return false
}

// ParsePaymentMethodType converts raw input into a PaymentMethodType.
func ParsePaymentMethodType(value string) (PaymentMethodType, error) {
	for _, candidate := range validPaymentMethodTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid payment method type %q", value)
}
