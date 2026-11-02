package core

import "strings"

// Go cyclomatic-complexity counting modes for
// quality_rules.cyclomatic_counting.
const (
	// CyclomaticCountingLegacy is the Go count codeguard used through v1.10.1:
	// 1, plus one per if, for, range, case clause, and comm clause (default
	// clauses included), with && and || not counted. It is the 1.x default so
	// a minor upgrade does not raise existing complexity numbers.
	CyclomaticCountingLegacy = "legacy"
	// CyclomaticCountingGocyclo matches gocyclo: 1, plus one per if, for,
	// range, non-default case or comm clause, and && or || operator. It is
	// the recommended mode and is expected to become the default in the next
	// major version.
	CyclomaticCountingGocyclo = "gocyclo"
)

// CyclomaticCountingModes lists the accepted quality_rules.cyclomatic_counting
// values, in the order shown to users.
func CyclomaticCountingModes() []string {
	return []string{CyclomaticCountingLegacy, CyclomaticCountingGocyclo}
}

// NormalizeCyclomaticCounting trims and lower-cases a configured counting
// mode and resolves an unset value to the 1.x default, legacy. It does not
// validate; config validation rejects unknown values before a scan runs.
func NormalizeCyclomaticCounting(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return CyclomaticCountingLegacy
	}
	return normalized
}
