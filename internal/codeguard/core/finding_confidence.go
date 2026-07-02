package core

import "strings"

// Finding confidence levels. Confidence expresses how likely a finding is to be
// a true positive, independent of its severity. An empty value means the check
// did not specify a confidence and consumers should treat it as medium.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// NormalizedConfidence maps a raw confidence value onto one of the known
// levels. Unknown or empty values normalize to "" (unspecified), which is
// treated as medium.
func NormalizedConfidence(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case ConfidenceHigh:
		return ConfidenceHigh
	case ConfidenceMedium:
		return ConfidenceMedium
	case ConfidenceLow:
		return ConfidenceLow
	default:
		return ""
	}
}
