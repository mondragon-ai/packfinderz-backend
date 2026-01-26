package enums

import "fmt"

// VendorOrderShippingStatus tracks shipping progress for a vendor order.
type VendorOrderShippingStatus string

const (
	VendorOrderShippingStatusPending    VendorOrderShippingStatus = "pending"
	VendorOrderShippingStatusDispatched VendorOrderShippingStatus = "dispatched"
	VendorOrderShippingStatusInTransit  VendorOrderShippingStatus = "in_transit"
	VendorOrderShippingStatusDelivered  VendorOrderShippingStatus = "delivered"
)

var validVendorOrderShippingStatuses = []VendorOrderShippingStatus{
	VendorOrderShippingStatusPending,
	VendorOrderShippingStatusDispatched,
	VendorOrderShippingStatusInTransit,
	VendorOrderShippingStatusDelivered,
}

// String implements fmt.Stringer.
func (v VendorOrderShippingStatus) String() string {
	return string(v)
}

// IsValid reports whether the value is a known VendorOrderShippingStatus.
func (v VendorOrderShippingStatus) IsValid() bool {
	for _, candidate := range validVendorOrderShippingStatuses {
		if candidate == v {
			return true
		}
	}
	return false
}

// ParseVendorOrderShippingStatus converts raw input into a VendorOrderShippingStatus.
func ParseVendorOrderShippingStatus(value string) (VendorOrderShippingStatus, error) {
	for _, candidate := range validVendorOrderShippingStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid vendor order shipping status %q", value)
}
