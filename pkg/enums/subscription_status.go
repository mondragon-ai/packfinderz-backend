package enums

import "fmt"

// SubscriptionStatus mirrors the billing provider's subscription state.
type SubscriptionStatus string

const (
	SubscriptionStatusTrialing          SubscriptionStatus = "trialing"
	SubscriptionStatusActive            SubscriptionStatus = "active"
	SubscriptionStatusPastDue           SubscriptionStatus = "past_due"
	SubscriptionStatusCanceled          SubscriptionStatus = "canceled"
	SubscriptionStatusIncomplete        SubscriptionStatus = "incomplete"
	SubscriptionStatusIncompleteExpired SubscriptionStatus = "incomplete_expired"
	SubscriptionStatusUnpaid            SubscriptionStatus = "unpaid"
)

var validSubscriptionStatuses = []SubscriptionStatus{
	SubscriptionStatusTrialing,
	SubscriptionStatusActive,
	SubscriptionStatusPastDue,
	SubscriptionStatusCanceled,
	SubscriptionStatusIncomplete,
	SubscriptionStatusIncompleteExpired,
	SubscriptionStatusUnpaid,
}

// String implements fmt.Stringer.
func (s SubscriptionStatus) String() string {
	return string(s)
}

// IsValid reports whether the value is known.
func (s SubscriptionStatus) IsValid() bool {
	for _, candidate := range validSubscriptionStatuses {
		if candidate == s {
			return true
		}
	}
	return false
}

// ParseSubscriptionStatus converts raw input into a SubscriptionStatus.
func ParseSubscriptionStatus(value string) (SubscriptionStatus, error) {
	for _, candidate := range validSubscriptionStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid subscription status %q", value)
}
