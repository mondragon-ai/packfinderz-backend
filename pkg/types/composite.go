package types

import (
	"errors"
	"fmt"
	"strings"
)

var errCompositeFieldCount = errors.New("composite: unexpected field count")

func quoteCompositeString(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r == '\\' || r == '"' {
			builder.WriteByte('\\')
		}
		builder.WriteRune(r)
	}
	return `"` + builder.String() + `"`
}

func quoteCompositeNullable(value *string) string {
	if value == nil {
		return "NULL"
	}
	return quoteCompositeString(*value)
}

func isCompositeNull(value string) bool {
	return strings.EqualFold(value, "NULL")
}

func parseComposite(raw string, expected int) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw[0] != '(' || raw[len(raw)-1] != ')' {
		return nil, fmt.Errorf("composite: invalid format %q", raw)
	}
	content := raw[1 : len(raw)-1]
	var fields []string
	var builder strings.Builder
	inQuotes := false
	escape := false

	for i := 0; i < len(content); i++ {
		ch := content[i]
		if escape {
			builder.WriteByte(ch)
			escape = false
			continue
		}

		switch ch {
		case '\\':
			escape = true
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				fields = append(fields, builder.String())
				builder.Reset()
				continue
			}
			builder.WriteByte(ch)
		default:
			builder.WriteByte(ch)
		}
	}

	fields = append(fields, builder.String())
	if expected > 0 && len(fields) != expected {
		return nil, fmt.Errorf("%w: got %d expected %d", errCompositeFieldCount, len(fields), expected)
	}
	return fields, nil
}

func newCompositeNullable(value string) *string {
	if isCompositeNull(value) {
		return nil
	}
	result := value
	return &result
}
