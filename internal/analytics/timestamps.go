package analytics

import "time"

// RevenueTimestamp selects the proper timestamp for revenue metrics.
// Order of preference is paidAt → cashCollectedAt → fallback.
func RevenueTimestamp(paidAt, cashCollectedAt *time.Time, fallback time.Time) time.Time {
	if paidAt != nil && !paidAt.IsZero() {
		return paidAt.UTC()
	}
	if cashCollectedAt != nil && !cashCollectedAt.IsZero() {
		return cashCollectedAt.UTC()
	}
	return fallback.UTC()
}
