package enums

import "fmt"

// RefundStatus tracks whether an order has been refunded.
type RefundStatus string

const (
	RefundStatusNone    RefundStatus = "none"
	RefundStatusPartial RefundStatus = "partial"
	RefundStatusFull    RefundStatus = "full"
)

var validRefundStatuses = []RefundStatus{
	RefundStatusNone,
	RefundStatusPartial,
	RefundStatusFull,
}

// String implements fmt.Stringer.
func (r RefundStatus) String() string {
	return string(r)
}

// IsValid reports whether the value is a known RefundStatus.
func (r RefundStatus) IsValid() bool {
	for _, candidate := range validRefundStatuses {
		if candidate == r {
			return true
		}
	}
	return false
}

// ParseRefundStatus converts raw input into a RefundStatus.
func ParseRefundStatus(value string) (RefundStatus, error) {
	for _, candidate := range validRefundStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid refund status %q", value)
}
