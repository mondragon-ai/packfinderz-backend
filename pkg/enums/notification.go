package enums

import "fmt"

// NotificationType maps to the notification_type enum in Postgres.
type NotificationType string

const (
	NotificationTypeSystemAnnouncement NotificationType = "system_announcement"
	NotificationTypeMarketUpdate       NotificationType = "market_update"
	NotificationTypeSecurityAlert      NotificationType = "security_alert"
	NotificationTypeOrderAlert         NotificationType = "order_alert"
	NotificationTypeCompliance         NotificationType = "compliance"
)

var validNotificationTypes = []NotificationType{
	NotificationTypeSystemAnnouncement,
	NotificationTypeMarketUpdate,
	NotificationTypeSecurityAlert,
	NotificationTypeOrderAlert,
	NotificationTypeCompliance,
}

// IsValid checks whether the given type matches the canonical enum.
func (n NotificationType) IsValid() bool {
	for _, candidate := range validNotificationTypes {
		if candidate == n {
			return true
		}
	}
	return false
}

// ParseNotificationType converts raw strings into NotificationType.
func ParseNotificationType(value string) (NotificationType, error) {
	for _, candidate := range validNotificationTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid notification type %q", value)
}
