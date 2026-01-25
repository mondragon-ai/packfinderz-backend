package enums

import "fmt"

// LineItemStatus tracks the acceptance and fulfillment state for a line item.
type LineItemStatus string

const (
	LineItemStatusPending   LineItemStatus = "pending"
	LineItemStatusAccepted  LineItemStatus = "accepted"
	LineItemStatusRejected  LineItemStatus = "rejected"
	LineItemStatusFulfilled LineItemStatus = "fulfilled"
	LineItemStatusHold      LineItemStatus = "hold"
)

var validLineItemStatuses = []LineItemStatus{
	LineItemStatusPending,
	LineItemStatusAccepted,
	LineItemStatusRejected,
	LineItemStatusFulfilled,
	LineItemStatusHold,
}

// String implements fmt.Stringer.
func (l LineItemStatus) String() string {
	return string(l)
}

// IsValid reports whether the value is a known LineItemStatus.
func (l LineItemStatus) IsValid() bool {
	for _, candidate := range validLineItemStatuses {
		if candidate == l {
			return true
		}
	}
	return false
}

// ParseLineItemStatus converts raw input into a LineItemStatus.
func ParseLineItemStatus(value string) (LineItemStatus, error) {
	for _, candidate := range validLineItemStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid line item status %q", value)
}
