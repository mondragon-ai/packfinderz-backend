package enums

import "fmt"

// MembershipStatus captures the lifecycle of a store membership.
type MembershipStatus string

const (
	MembershipStatusInvited MembershipStatus = "invited"
	MembershipStatusActive  MembershipStatus = "active"
	MembershipStatusRemoved MembershipStatus = "removed"
	MembershipStatusPending MembershipStatus = "pending"
)

var validMembershipStatuses = []MembershipStatus{
	MembershipStatusInvited,
	MembershipStatusActive,
	MembershipStatusRemoved,
	MembershipStatusPending,
}

// String implements fmt.Stringer.
func (m MembershipStatus) String() string {
	return string(m)
}

// IsValid reports whether the value matches a known MembershipStatus.
func (m MembershipStatus) IsValid() bool {
	for _, candidate := range validMembershipStatuses {
		if candidate == m {
			return true
		}
	}
	return false
}

// ParseMembershipStatus converts raw input into a MembershipStatus.
func ParseMembershipStatus(value string) (MembershipStatus, error) {
	for _, candidate := range validMembershipStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid membership status %q", value)
}
