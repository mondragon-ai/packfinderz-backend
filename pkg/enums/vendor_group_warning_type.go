package enums

import "fmt"

// VendorGroupWarningType enumerates warning reasons stored on vendor groups.
type VendorGroupWarningType string

const (
	VendorGroupWarningTypeVendorInvalid   VendorGroupWarningType = "vendor_invalid"
	VendorGroupWarningTypeVendorSuspended VendorGroupWarningType = "vendor_suspended"
	VendorGroupWarningTypeLicenseInvalid  VendorGroupWarningType = "license_invalid"
	VendorGroupWarningTypeInvalidPromo    VendorGroupWarningType = "invalid_promo"
)

var validVendorGroupWarningTypes = []VendorGroupWarningType{
	VendorGroupWarningTypeVendorInvalid,
	VendorGroupWarningTypeVendorSuspended,
	VendorGroupWarningTypeLicenseInvalid,
	VendorGroupWarningTypeInvalidPromo,
}

// String implements fmt.Stringer.
func (v VendorGroupWarningType) String() string {
	return string(v)
}

// IsValid reports whether the value is known.
func (v VendorGroupWarningType) IsValid() bool {
	for _, candidate := range validVendorGroupWarningTypes {
		if candidate == v {
			return true
		}
	}
	return false
}

// ParseVendorGroupWarningType converts raw input into a VendorGroupWarningType.
func ParseVendorGroupWarningType(value string) (VendorGroupWarningType, error) {
	for _, candidate := range validVendorGroupWarningTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid vendor group warning type %q", value)
}
