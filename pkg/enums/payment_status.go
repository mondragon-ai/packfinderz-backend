package enums

import "fmt"

// PaymentStatus tracks the lifecycle of a payment intent.
type PaymentStatus string

const (
	PaymentStatusUnpaid   PaymentStatus = "unpaid"
	PaymentStatusPending  PaymentStatus = "pending"
	PaymentStatusSettled  PaymentStatus = "settled"
	PaymentStatusPaid     PaymentStatus = "paid"
	PaymentStatusFailed   PaymentStatus = "failed"
	PaymentStatusRejected PaymentStatus = "rejected"
)

var validPaymentStatuses = []PaymentStatus{
	PaymentStatusUnpaid,
	PaymentStatusPending,
	PaymentStatusSettled,
	PaymentStatusPaid,
	PaymentStatusFailed,
	PaymentStatusRejected,
}

// String implements fmt.Stringer.
func (p PaymentStatus) String() string {
	return string(p)
}

// IsValid reports whether the value is a known PaymentStatus.
func (p PaymentStatus) IsValid() bool {
	for _, candidate := range validPaymentStatuses {
		if candidate == p {
			return true
		}
	}
	return false
}

// ParsePaymentStatus converts raw input into a PaymentStatus.
func ParsePaymentStatus(value string) (PaymentStatus, error) {
	for _, candidate := range validPaymentStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid payment status %q", value)
}
