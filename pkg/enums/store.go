package enums

import "fmt"

// StoreType represents the canonical store_type enum in Postgres.
type StoreType string

const (
	StoreTypeBuyer  StoreType = "buyer"
	StoreTypeVendor StoreType = "vendor"
)

var validStoreTypes = []StoreType{
	StoreTypeBuyer,
	StoreTypeVendor,
}

// String implements fmt.Stringer.
func (s StoreType) String() string {
	return string(s)
}

// IsValid reports whether the value is a known StoreType.
func (s StoreType) IsValid() bool {
	for _, candidate := range validStoreTypes {
		if candidate == s {
			return true
		}
	}
	return false
}

// ParseStoreType converts raw input into a StoreType.
func ParseStoreType(value string) (StoreType, error) {
	for _, candidate := range validStoreTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid store type %q", value)
}

// KYCStatus captures the store-level verification workflow.
type KYCStatus string

const (
	KYCStatusPendingVerification KYCStatus = "pending_verification"
	KYCStatusVerified            KYCStatus = "verified"
	KYCStatusRejected            KYCStatus = "rejected"
	KYCStatusExpired             KYCStatus = "expired"
	KYCStatusSuspended           KYCStatus = "suspended"
)

var validKYCStatuses = []KYCStatus{
	KYCStatusPendingVerification,
	KYCStatusVerified,
	KYCStatusRejected,
	KYCStatusExpired,
	KYCStatusSuspended,
}

// String implements fmt.Stringer.
func (s KYCStatus) String() string {
	return string(s)
}

// IsValid reports whether the value matches the GORM enum.
func (s KYCStatus) IsValid() bool {
	for _, candidate := range validKYCStatuses {
		if candidate == s {
			return true
		}
	}
	return false
}

// ParseKYCStatus converts raw input into a KYCStatus.
func ParseKYCStatus(value string) (KYCStatus, error) {
	for _, candidate := range validKYCStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid KYC status %q", value)
}
