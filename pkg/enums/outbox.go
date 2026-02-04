package enums

import "fmt"

// OutboxAggregateType maps to the aggregate_type enum in Postgres.
type OutboxAggregateType string

const (
	AggregateVendorOrder   OutboxAggregateType = "vendor_order"
	AggregateCheckoutGroup OutboxAggregateType = "checkout_group"
	AggregateLicense       OutboxAggregateType = "license"
	AggregateStore         OutboxAggregateType = "store"
	AggregateMedia         OutboxAggregateType = "media"
	AggregateLedgerEvent   OutboxAggregateType = "ledger_event"
	AggregateNotification  OutboxAggregateType = "notification"
	AggregateAd            OutboxAggregateType = "ad"
)

var validAggregateTypes = []OutboxAggregateType{
	AggregateVendorOrder,
	AggregateCheckoutGroup,
	AggregateLicense,
	AggregateStore,
	AggregateMedia,
	AggregateLedgerEvent,
	AggregateNotification,
	AggregateAd,
}

// IsValid reports whether the value matches the canonical aggregate_type enum.
func (a OutboxAggregateType) IsValid() bool {
	for _, candidate := range validAggregateTypes {
		if candidate == a {
			return true
		}
	}
	return false
}

// ParseOutboxAggregateType converts raw input into OutboxAggregateType.
func ParseOutboxAggregateType(value string) (OutboxAggregateType, error) {
	for _, candidate := range validAggregateTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid aggregate type %q", value)
}

// OutboxEventType maps to the event_type enum in Postgres.
type OutboxEventType string

const (
	EventOrderCreated          OutboxEventType = "order_created"
	EventOrderStateChanged     OutboxEventType = "order_state_changed"
	EventLineItemStateChanged  OutboxEventType = "line_item_state_changed"
	EventLicenseStatusChanged  OutboxEventType = "license_status_changed"
	EventLicenseExpiringSoon   OutboxEventType = "license_expiring_soon"
	EventLicenseExpired        OutboxEventType = "license_expired"
	EventMediaUploaded         OutboxEventType = "media_uploaded"
	EventPaymentSettled        OutboxEventType = "payment_settled"
	EventCashCollected         OutboxEventType = "cash_collected"
	EventPaymentFailed         OutboxEventType = "payment_failed"
	EventPaymentRejected       OutboxEventType = "payment_rejected"
	EventVendorPayoutRecorded  OutboxEventType = "vendor_payout_recorded"
	EventNotificationRequested OutboxEventType = "notification_requested"
	EventOrderExpired          OutboxEventType = "order_expired"
	EventOrderPendingNudge     OutboxEventType = "order_pending_nudge"
	EventOrderCanceled         OutboxEventType = "order_canceled"
	EventOrderRetried          OutboxEventType = "order_retried"
	EventOrderPaid             OutboxEventType = "order_paid"
	EventOrderDecided          OutboxEventType = "order_decided"
	EventOrderReadyForDispatch OutboxEventType = "order_ready_for_dispatch"
	EventReservationReleased   OutboxEventType = "reservation_released"
	EventAdCreated             OutboxEventType = "ad_created"
	EventAdUpdated             OutboxEventType = "ad_updated"
	EventAdPaused              OutboxEventType = "ad_paused"
	EventAdActivated           OutboxEventType = "ad_activated"
	EventAdExpired             OutboxEventType = "ad_expired"
	EventAdDailyRollupReady    OutboxEventType = "ad_daily_rollup_ready"
	EventCheckoutConverted     OutboxEventType = "checkout_converted"
)

var validOutboxEventTypes = []OutboxEventType{
	EventOrderCreated,
	EventOrderStateChanged,
	EventLineItemStateChanged,
	EventLicenseStatusChanged,
	EventLicenseExpiringSoon,
	EventLicenseExpired,
	EventMediaUploaded,
	EventPaymentSettled,
	EventCashCollected,
	EventPaymentFailed,
	EventPaymentRejected,
	EventVendorPayoutRecorded,
	EventNotificationRequested,
	EventOrderExpired,
	EventOrderPendingNudge,
	EventOrderCanceled,
	EventOrderRetried,
	EventOrderPaid,
	EventOrderDecided,
	EventOrderReadyForDispatch,
	EventReservationReleased,
	EventAdCreated,
	EventAdUpdated,
	EventAdPaused,
	EventAdActivated,
	EventAdExpired,
	EventAdDailyRollupReady,
	EventCheckoutConverted,
}

// IsValid reports whether the value matches the canonical event_type enum.
func (e OutboxEventType) IsValid() bool {
	for _, candidate := range validOutboxEventTypes {
		if candidate == e {
			return true
		}
	}
	return false
}

// ParseOutboxEventType converts raw input into OutboxEventType.
func ParseOutboxEventType(value string) (OutboxEventType, error) {
	for _, candidate := range validOutboxEventTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid event type %q", value)
}
