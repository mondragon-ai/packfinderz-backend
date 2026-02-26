package enums

import (
	"fmt"
	"strings"
)

// ReviewType distinguishes the kind of entity being reviewed.
type ReviewType string

const (
	ReviewTypeStore   ReviewType = "store"
	ReviewTypeProduct ReviewType = "product"
)

var validReviewTypes = []ReviewType{
	ReviewTypeStore,
	ReviewTypeProduct,
}

// String implements fmt.Stringer.
func (r ReviewType) String() string {
	return string(r)
}

// IsValid reports whether the value matches a known review type.
func (r ReviewType) IsValid() bool {
	for _, candidate := range validReviewTypes {
		if candidate == r {
			return true
		}
	}
	return false
}

// ParseReviewType normalizes raw input into a ReviewType constant.
func ParseReviewType(value string) (ReviewType, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range validReviewTypes {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid review type %q", value)
}
