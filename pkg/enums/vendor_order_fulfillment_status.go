package enums

import "fmt"

// VendorOrderFulfillmentStatus tracks fulfillment progress for a vendor order.
type VendorOrderFulfillmentStatus string

const (
	VendorOrderFulfillmentStatusPending   VendorOrderFulfillmentStatus = "pending"
	VendorOrderFulfillmentStatusPartial   VendorOrderFulfillmentStatus = "partial"
	VendorOrderFulfillmentStatusFulfilled VendorOrderFulfillmentStatus = "fulfilled"
)

var validVendorOrderFulfillmentStatuses = []VendorOrderFulfillmentStatus{
	VendorOrderFulfillmentStatusPending,
	VendorOrderFulfillmentStatusPartial,
	VendorOrderFulfillmentStatusFulfilled,
}

// String implements fmt.Stringer.
func (v VendorOrderFulfillmentStatus) String() string {
	return string(v)
}

// IsValid reports whether the value is a known VendorOrderFulfillmentStatus.
func (v VendorOrderFulfillmentStatus) IsValid() bool {
	for _, candidate := range validVendorOrderFulfillmentStatuses {
		if candidate == v {
			return true
		}
	}
	return false
}

// ParseVendorOrderFulfillmentStatus converts raw input into a VendorOrderFulfillmentStatus.
func ParseVendorOrderFulfillmentStatus(value string) (VendorOrderFulfillmentStatus, error) {
	for _, candidate := range validVendorOrderFulfillmentStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid vendor order fulfillment status %q", value)
}
