package validators

import "strings"

func SanitizeString(input string, maxLen int) string {
	trimmed := strings.TrimSpace(input)
	if maxLen > 0 && len(trimmed) > maxLen {
		return trimmed[:maxLen]
	}
	return trimmed
}
