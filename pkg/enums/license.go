package enums

import "fmt"

// LicenseStatus maps to the license_status enum in Postgres.
type LicenseStatus string

const (
	LicenseStatusPending  LicenseStatus = "pending"
	LicenseStatusVerified LicenseStatus = "verified"
	LicenseStatusRejected LicenseStatus = "rejected"
	LicenseStatusExpired  LicenseStatus = "expired"
)

var validLicenseStatuses = []LicenseStatus{
	LicenseStatusPending,
	LicenseStatusVerified,
	LicenseStatusRejected,
	LicenseStatusExpired,
}

// String implements fmt.Stringer.
func (l LicenseStatus) String() string {
	return string(l)
}

// IsValid reports whether the value matches the canonical license_status enum.
func (l LicenseStatus) IsValid() bool {
	for _, candidate := range validLicenseStatuses {
		if candidate == l {
			return true
		}
	}
	return false
}

// ParseLicenseStatus converts raw input into LicenseStatus.
func ParseLicenseStatus(value string) (LicenseStatus, error) {
	for _, candidate := range validLicenseStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid license status %q", value)
}

// LicenseType maps to the license_type enum in Postgres.
type LicenseType string

const (
	LicenseTypeProducer   LicenseType = "producer"
	LicenseTypeGrower     LicenseType = "grower"
	LicenseTypeDispensary LicenseType = "dispensary"
	LicenseTypeMerchant   LicenseType = "merchant"
)

var validLicenseTypes = []LicenseType{
	LicenseTypeProducer,
	LicenseTypeGrower,
	LicenseTypeDispensary,
	LicenseTypeMerchant,
}

// String implements fmt.Stringer.
func (l LicenseType) String() string {
	return string(l)
}

// IsValid reports whether the value matches the canonical license_type enum.
func (l LicenseType) IsValid() bool {
	for _, candidate := range validLicenseTypes {
		if candidate == l {
			return true
		}
	}
	return false
}

// ParseLicenseType converts raw input into LicenseType.
func ParseLicenseType(value string) (LicenseType, error) {
	for _, candidate := range validLicenseTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid license type %q", value)
}
