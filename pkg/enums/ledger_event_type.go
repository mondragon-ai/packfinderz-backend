package enums

import "fmt"

// LedgerEventType maps to the ledger_event_type_enum enum in Postgres.
type LedgerEventType string

const (
	LedgerEventTypeCashCollected LedgerEventType = "cash_collected"
	LedgerEventTypeVendorPayout  LedgerEventType = "vendor_payout"
	LedgerEventTypeAdjustment    LedgerEventType = "adjustment"
	LedgerEventTypeRefund        LedgerEventType = "refund"
)

var validLedgerEventTypes = []LedgerEventType{
	LedgerEventTypeCashCollected,
	LedgerEventTypeVendorPayout,
	LedgerEventTypeAdjustment,
	LedgerEventTypeRefund,
}

// IsValid reports whether the value matches the canonical ledger event enum.
func (t LedgerEventType) IsValid() bool {
	for _, candidate := range validLedgerEventTypes {
		if candidate == t {
			return true
		}
	}
	return false
}

// ParseLedgerEventType converts raw input into LedgerEventType.
func ParseLedgerEventType(value string) (LedgerEventType, error) {
	for _, candidate := range validLedgerEventTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid ledger event type %q", value)
}
