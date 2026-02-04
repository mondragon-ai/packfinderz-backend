package enums

import "fmt"

// VendorOrderStatus tracks the lifecycle of a vendor order.
type VendorOrderStatus string

const (
	VendorOrderStatusCreatedPending    VendorOrderStatus = "created_pending"
	VendorOrderStatusAccepted          VendorOrderStatus = "accepted"
	VendorOrderStatusPartiallyAccepted VendorOrderStatus = "partially_accepted"
	VendorOrderStatusRejected          VendorOrderStatus = "rejected"
	VendorOrderStatusFulfilled         VendorOrderStatus = "fulfilled"
	VendorOrderStatusReadyForDispatch  VendorOrderStatus = "ready_for_dispatch"
	VendorOrderStatusHold              VendorOrderStatus = "hold"
	VendorOrderStatusHoldForPickup     VendorOrderStatus = "hold_for_pickup"
	VendorOrderStatusInTransit         VendorOrderStatus = "in_transit"
	VendorOrderStatusDelivered         VendorOrderStatus = "delivered"
	VendorOrderStatusClosed            VendorOrderStatus = "closed"
	VendorOrderStatusCanceled          VendorOrderStatus = "canceled"
	VendorOrderStatusExpired           VendorOrderStatus = "expired"
)

var validVendorOrderStatuses = []VendorOrderStatus{
	VendorOrderStatusCreatedPending,
	VendorOrderStatusAccepted,
	VendorOrderStatusPartiallyAccepted,
	VendorOrderStatusRejected,
	VendorOrderStatusFulfilled,
	VendorOrderStatusReadyForDispatch,
	VendorOrderStatusHold,
	VendorOrderStatusHoldForPickup,
	VendorOrderStatusInTransit,
	VendorOrderStatusDelivered,
	VendorOrderStatusClosed,
	VendorOrderStatusCanceled,
	VendorOrderStatusExpired,
}

// String implements fmt.Stringer.
func (v VendorOrderStatus) String() string {
	return string(v)
}

// IsValid reports whether the value is a known VendorOrderStatus.
func (v VendorOrderStatus) IsValid() bool {
	for _, candidate := range validVendorOrderStatuses {
		if candidate == v {
			return true
		}
	}
	return false
}

// ParseVendorOrderStatus converts raw input into a VendorOrderStatus.
func ParseVendorOrderStatus(value string) (VendorOrderStatus, error) {
	for _, candidate := range validVendorOrderStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid vendor order status %q", value)
}
