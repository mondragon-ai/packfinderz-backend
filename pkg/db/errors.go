package db

import "strings"

// IsUniqueViolation reports whether the provided error references a Postgres
// unique violation constraint. When constraintName is provided, the helper looks
// for the constraint text in the error message.
func IsUniqueViolation(err error, constraintName string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if constraintName != "" {
		return strings.Contains(msg, constraintName)
	}
	return strings.Contains(msg, "duplicate key value")
}
