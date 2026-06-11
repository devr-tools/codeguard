package support

func NormalizedSeverity(level string) string {
	switch level {
	case "fail", "warn", "pass":
		return level
	default:
		return "warn"
	}
}
