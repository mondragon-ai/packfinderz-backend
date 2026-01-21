package enums

import "fmt"

// MediaStatus describes the lifecycle state of media uploads.
type MediaStatus string

const (
	MediaStatusPending         MediaStatus = "pending"
	MediaStatusUploaded        MediaStatus = "uploaded"
	MediaStatusProcessing      MediaStatus = "processing"
	MediaStatusReady           MediaStatus = "ready"
	MediaStatusFailed          MediaStatus = "failed"
	MediaStatusDeleteRequested MediaStatus = "delete_requested"
	MediaStatusDeleted         MediaStatus = "deleted"
	MediaStatusDeleteFailed    MediaStatus = "delete_failed"
)

var validMediaStatuses = []MediaStatus{
	MediaStatusPending,
	MediaStatusUploaded,
	MediaStatusProcessing,
	MediaStatusReady,
	MediaStatusFailed,
	MediaStatusDeleteRequested,
	MediaStatusDeleted,
	MediaStatusDeleteFailed,
}

// String returns the literal string for the status.
func (m MediaStatus) String() string {
	return string(m)
}

// IsValid reports whether the status is known.
func (m MediaStatus) IsValid() bool {
	for _, candidate := range validMediaStatuses {
		if candidate == m {
			return true
		}
	}
	return false
}

// ParseMediaStatus converts raw input into a MediaStatus.
func ParseMediaStatus(value string) (MediaStatus, error) {
	for _, candidate := range validMediaStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid media status %q", value)
}
