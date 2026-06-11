package runner

import "strings"

func normalizedSeverity(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "fail":
		return "fail"
	default:
		return "warn"
	}
}
