package security

import "regexp"

// compileDynamicPattern compiles expressions built from runtime-derived text.
// Do not cache these expressions globally: aliases and module names originate
// in scanned source, so an unbounded process-wide cache would retain
// attacker-controlled entries across scans in long-lived processes.
func compileDynamicPattern(expr string) *regexp.Regexp {
	return regexp.MustCompile(expr)
}
