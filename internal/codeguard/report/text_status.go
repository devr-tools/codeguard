package report

import (
	"os"
	"strings"
)

func renderStatus(status string, includeIcon bool) string {
	label := statusLabel(status)
	if includeIcon {
		label = statusIcon(status) + " " + label
	}
	return colorize(label, statusColor(status))
}

func renderStatusBadge(status string) string {
	return colorize("["+statusLabel(status)+"]", statusColor(status))
}

func statusLabel(status string) string {
	return strings.ToUpper(strings.TrimSpace(status))
}

func statusIcon(status string) string {
	switch statusLabel(status) {
	case "PASS":
		return "✅"
	case "WARN":
		return "⚠️"
	case "FAIL":
		return "❌"
	default:
		return "•"
	}
}

func statusColor(status string) string {
	switch statusLabel(status) {
	case "PASS":
		return "32"
	case "WARN":
		return "33"
	case "FAIL":
		return "31"
	default:
		return ""
	}
}

func colorize(value string, color string) string {
	if color == "" || noColor() {
		return value
	}
	return "\x1b[" + color + "m" + value + "\x1b[0m"
}

func noColor() bool {
	return strings.TrimSpace(os.Getenv("NO_COLOR")) != ""
}
