package router

import "strings"

// stringPtr returns a trimmed pointer or nil when the input is empty.
func stringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// int64Ptr returns a pointer to the provided int64 value.
func int64Ptr(value int64) *int64 {
	return &value
}

// float64Ptr returns a pointer to the provided float64 value.
func float64Ptr(value float64) *float64 {
	return &value
}
