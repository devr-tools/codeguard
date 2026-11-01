package report

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// securitySeverityScores maps a security finding's level and confidence onto a
// CVSS-like 0-10 score. GitHub code scanning buckets the score into its
// severity label: above 9.0 critical, 7.0-8.9 high, 4.0-6.9 medium, below 4.0
// low. A fail-level finding backed by structured evidence (high confidence,
// e.g. a source-to-sink taint flow) is critical; a warn-level review item
// never rises above medium. The table is documented in docs/checks.md
// ("SARIF output").
var securitySeverityScores = map[string]map[string]float64{
	"fail": {core.ConfidenceHigh: 9.3, core.ConfidenceMedium: 8.1, core.ConfidenceLow: 6.5},
	"warn": {core.ConfidenceHigh: 6.1, core.ConfidenceMedium: 5.0, core.ConfidenceLow: 3.1},
}

// securitySeverityScore returns the score for one finding level and
// confidence. Unknown levels score as warn and unspecified confidence as
// medium, matching how the rest of codeguard treats those values.
func securitySeverityScore(level string, confidence string) float64 {
	byConfidence, ok := securitySeverityScores[strings.ToLower(strings.TrimSpace(level))]
	if !ok {
		byConfidence = securitySeverityScores["warn"]
	}
	return byConfidence[confidenceOrMedium(confidence)]
}

func formatSecuritySeverity(score float64) string {
	return strconv.FormatFloat(score, 'f', 1, 64)
}

func confidenceOrMedium(confidence string) string {
	if normalized := core.NormalizedConfidence(confidence); normalized != "" {
		return normalized
	}
	return core.ConfidenceMedium
}

// sarifLevel maps a codeguard level onto a SARIF result level.
func sarifLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "fail":
		return "error"
	case "warn":
		return "warning"
	default:
		return "note"
	}
}

// isSecurityRule reports whether a rule should be classified as security in
// GitHub code scanning: every rule in the Security section, plus rules in
// other sections (CI workflow security, vulnerable dependencies) that carry a
// CWE mapping.
func isSecurityRule(meta core.RuleMetadata, ruleID string) bool {
	if strings.EqualFold(meta.Section, "security") || strings.HasPrefix(ruleID, "security.") {
		return true
	}
	return len(meta.CWE) > 0
}

// cweTag renders "CWE-78" as the CodeQL-compatible tag "external/cwe/cwe-078",
// so a GitHub `tag:external/cwe/cwe-078` filter groups codeguard alerts with
// CodeQL's for the same weakness.
func cweTag(cwe string) (string, bool) {
	number, ok := cweNumber(cwe)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("external/cwe/cwe-%03d", number), true
}

func cweNumber(cwe string) (int, bool) {
	trimmed := strings.TrimSpace(cwe)
	if len(trimmed) < 4 || !strings.EqualFold(trimmed[:4], "cwe-") {
		return 0, false
	}
	number, err := strconv.Atoi(trimmed[4:])
	if err != nil || number <= 0 {
		return 0, false
	}
	return number, true
}

// sectionTag turns a section id or display name ("supply_chain",
// "Supply Chain") into a lowercase tag ("supply-chain").
func sectionTag(section string) string {
	fields := strings.FieldsFunc(strings.ToLower(section), func(r rune) bool {
		return r == ' ' || r == '_' || r == '-'
	})
	return strings.Join(fields, "-")
}
