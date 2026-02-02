package enums

import "fmt"

// AnalyticsEventType is the canonical event_type for analytics routing.
type AnalyticsEventType string

const (
	AnalyticsEventOrderCreated          AnalyticsEventType = "order_created"
	AnalyticsEventOrderPaid             AnalyticsEventType = "order_paid"
	AnalyticsEventCashCollected         AnalyticsEventType = "cash_collected"
	AnalyticsEventOrderCanceled         AnalyticsEventType = "order_canceled"
	AnalyticsEventOrderExpired          AnalyticsEventType = "order_expired"
	AnalyticsEventRefundInitiated       AnalyticsEventType = "refund_initiated"
	AnalyticsEventAdImpression          AnalyticsEventType = "ad_impression"
	AnalyticsEventAdClick               AnalyticsEventType = "ad_click"
	AnalyticsEventAdDailyChargeRecorded AnalyticsEventType = "ad_daily_charge_recorded"
)

var validAnalyticsEventTypes = []AnalyticsEventType{
	AnalyticsEventOrderCreated,
	AnalyticsEventOrderPaid,
	AnalyticsEventCashCollected,
	AnalyticsEventOrderCanceled,
	AnalyticsEventOrderExpired,
	AnalyticsEventRefundInitiated,
	AnalyticsEventAdImpression,
	AnalyticsEventAdClick,
	AnalyticsEventAdDailyChargeRecorded,
}

// IsValid reports whether the value matches the canonical analytics event_type enum.
func (a AnalyticsEventType) IsValid() bool {
	for _, candidate := range validAnalyticsEventTypes {
		if candidate == a {
			return true
		}
	}
	return false
}

// ParseAnalyticsEventType converts the raw string to AnalyticsEventType.
func ParseAnalyticsEventType(value string) (AnalyticsEventType, error) {
	for _, candidate := range validAnalyticsEventTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid analytics event type %q", value)
}
