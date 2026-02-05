package enums

import "fmt"

// PlanStatus tracks the lifecycle state of a billing plan.
type PlanStatus string

const (
	PlanStatusActive     PlanStatus = "active"
	PlanStatusDeprecated PlanStatus = "deprecated"
	PlanStatusHidden     PlanStatus = "hidden"
)

var validPlanStatuses = []PlanStatus{
	PlanStatusActive,
	PlanStatusDeprecated,
	PlanStatusHidden,
}

// String implements fmt.Stringer.
func (p PlanStatus) String() string {
	return string(p)
}

// IsValid reports whether the value is a known PlanStatus.
func (p PlanStatus) IsValid() bool {
	for _, candidate := range validPlanStatuses {
		if candidate == p {
			return true
		}
	}
	return false
}

// ParsePlanStatus converts raw input into a PlanStatus.
func ParsePlanStatus(value string) (PlanStatus, error) {
	for _, candidate := range validPlanStatuses {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid plan status %q", value)
}
