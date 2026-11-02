package core

import (
	"fmt"
	"strings"
)

// Exit policies accepted by output.fail_on and the scan -fail-on flag. They
// choose which section status makes a scan exit non-zero.
const (
	// FailOnFail exits non-zero when any section fails. It is the default.
	FailOnFail = "fail"
	// FailOnWarn exits non-zero when any section warns or fails.
	FailOnWarn = "warn"
	// FailOnNone never exits non-zero because of findings. Operational
	// failures still do; see ReportGateError.
	FailOnNone = "none"
)

// FailOnValues lists the accepted exit policies in documentation order.
func FailOnValues() []string {
	return []string{FailOnFail, FailOnWarn, FailOnNone}
}

// NormalizedFailOn returns the canonical exit policy for value. An empty value
// resolves to FailOnFail. ok is false for an unknown policy.
func NormalizedFailOn(value string) (policy string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", FailOnFail:
		return FailOnFail, true
	case FailOnWarn:
		return FailOnWarn, true
	case FailOnNone:
		return FailOnNone, true
	default:
		return "", false
	}
}

// ReportGateError reports whether a scan result should fail the caller's gate
// under the given exit policy, returning nil when it should pass.
//
// Operational failures are checked first and are never relaxed by the policy:
// a fail-level operational diagnostic (scan.section.panic, scan.target-walk,
// a misconfigured required tool, ...) means part of the repository was not
// checked, so even FailOnNone must not let it read as a pass. Only
// finding-driven section status is subject to the policy. An unknown policy is
// treated as FailOnFail; config validation rejects it before a scan runs.
func ReportGateError(report Report, failOn string) error {
	if section, diagnostic, found := firstOperationalFailure(report); found {
		return fmt.Errorf("scan incomplete: %s reported %s: %s", section, diagnostic.ID, diagnostic.Message)
	}
	policy, ok := NormalizedFailOn(failOn)
	if !ok {
		policy = FailOnFail
	}
	switch policy {
	case FailOnNone:
		return nil
	case FailOnWarn:
		if report.Summary.FailedSections > 0 || report.Summary.WarnedSections > 0 {
			return fmt.Errorf("one or more sections failed or warned (fail-on: warn)")
		}
		return nil
	default:
		if report.Summary.FailedSections > 0 {
			return fmt.Errorf("one or more sections failed")
		}
		return nil
	}
}

func firstOperationalFailure(report Report) (string, Diagnostic, bool) {
	for _, section := range report.Sections {
		for _, diagnostic := range section.Diagnostics {
			if diagnostic.Operational && strings.EqualFold(strings.TrimSpace(diagnostic.Level), string(StatusFail)) {
				name := section.Name
				if name == "" {
					name = section.ID
				}
				return name, diagnostic, true
			}
		}
	}
	return "", Diagnostic{}, false
}
