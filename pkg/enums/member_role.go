package enums

import "fmt"

// MemberRole represents a store-level permissions role.
type MemberRole string

const (
	MemberRoleOwner   MemberRole = "owner"
	MemberRoleAdmin   MemberRole = "admin"
	MemberRoleManager MemberRole = "manager"
	MemberRoleViewer  MemberRole = "viewer"
	MemberRoleAgent   MemberRole = "agent"
	MemberRoleStaff   MemberRole = "staff"
	MemberRoleOps     MemberRole = "ops"
)

var validMemberRoles = []MemberRole{
	MemberRoleOwner,
	MemberRoleAdmin,
	MemberRoleManager,
	MemberRoleViewer,
	MemberRoleAgent,
	MemberRoleStaff,
	MemberRoleOps,
}

// String implements fmt.Stringer.
func (m MemberRole) String() string {
	return string(m)
}

// IsValid reports whether the value is a known MemberRole.
func (m MemberRole) IsValid() bool {
	for _, candidate := range validMemberRoles {
		if candidate == m {
			return true
		}
	}
	return false
}

// ParseMemberRole converts raw input into a MemberRole.
func ParseMemberRole(value string) (MemberRole, error) {
	for _, candidate := range validMemberRoles {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid member role %q", value)
}
