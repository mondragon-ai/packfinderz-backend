package enums

import "fmt"

// VendorGroupStatus tracks the validation state of a vendor-level quote aggregate.
type VendorGroupStatus string

const (
	VendorGroupStatusOK      VendorGroupStatus = "ok"
	VendorGroupStatusInvalid VendorGroupStatus = "invalid"
)

var validVendorGroupStatuses = []VendorGroupStatus{
	VendorGroupStatusOK,
	VendorGroupStatusInvalid,
}

// String implements fmt.Stringer.
func (v VendorGroupStatus) String() string {
	return string(v)
}

// IsValid reports whether the value is known.
func (v VendorGroupStatus) IsValid() bool {
	for _, candidate := range validVendorGroupStatuses {
		if candidate == v {
			return true
		}
	}
	return false
}

// ParseVendorGroupStatus converts raw input into a VendorGroupStatus.
func ParseVendorGroupStatus(value string) (VendorGroupStatus, error) {
	for _, candidate := range validVendorGroupStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid vendor group status %q", value)
}
