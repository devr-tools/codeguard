package support

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type inlineSuppression struct {
	ruleID  string
	line    int
	expires string
}

var inlineIgnorePattern = regexp.MustCompile(`codeguard:ignore\s+([a-z0-9._*\-]+)(?:\s+until\s+(\d{4}-\d{2}-\d{2}))?`)

// Suppression reasons returned by IsSuppressed, keyed on by the rule-stats
// collector to attribute suppressed findings to their mechanism.
const (
	SuppressionReasonBaseline = "baseline"
	SuppressionReasonWaiver   = "waiver"
	SuppressionReasonInline   = "inline suppression"
)

func IsSuppressed(sc Context, finding core.Finding) (bool, string) {
	if sc.Baseline != nil {
		if _, ok := sc.Baseline[finding.Fingerprint]; ok {
			return true, SuppressionReasonBaseline
		}
		// The context fingerprint deliberately omits the line number, so two
		// identical findings in the same file (same rule, same normalized
		// surrounding source, different locations) collide on it. For
		// suppression that is acceptable: baselining one occurrence of a
		// duplicated snippet also baselines its identical twins, and any real
		// change to the offending code alters the context and resurfaces the
		// finding. Baseline files written before context fingerprints existed
		// carry legacy-only entries and are matched by the check above.
		if finding.ContextFingerprint != "" {
			if _, ok := sc.Baseline[finding.ContextFingerprint]; ok {
				return true, "baseline"
			}
		}
	}
	if waiverMatches(sc, finding) {
		return true, SuppressionReasonWaiver
	}
	fullPath := findingFullPath(sc, finding.Path)
	if fullPath == "" {
		return false, ""
	}
	directives, err := parseInlineSuppressions(fullPath)
	if err != nil {
		return false, ""
	}
	if inlineSuppressionMatches(sc, finding, directives) {
		return true, SuppressionReasonInline
	}
	return false, ""
}

func waiverMatches(sc Context, finding core.Finding) bool {
	for _, waiver := range sc.Cfg.Waivers {
		if waiver.Rule != "*" && waiver.Rule != finding.RuleID {
			continue
		}
		if waiver.Path != "" && !MatchPattern(waiver.Path, finding.Path) {
			continue
		}
		if suppressionExpired(waiver.ExpiresOn, sc.Today) {
			continue
		}
		return true
	}
	return false
}

func findingFullPath(sc Context, rel string) string {
	if rel == "" {
		return ""
	}
	for _, target := range sc.Cfg.Targets {
		candidate := filepath.Join(target.Path, filepath.FromSlash(rel))
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func inlineSuppressionMatches(sc Context, finding core.Finding, directives []inlineSuppression) bool {
	for _, directive := range directives {
		if directive.ruleID != "*" && directive.ruleID != finding.RuleID {
			continue
		}
		if suppressionExpired(directive.expires, sc.Today) {
			continue
		}
		if finding.Line == 0 || finding.Line == directive.line || finding.Line == directive.line+1 {
			return true
		}
	}
	return false
}

func suppressionExpired(expires string, today time.Time) bool {
	if expires == "" {
		return false
	}
	parsed, err := time.Parse("2006-01-02", expires)
	return err == nil && parsed.Before(DateOnly(today))
}

func parseInlineSuppressions(path string) ([]inlineSuppression, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path resolved via findingFullPath against the scan context
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	out := make([]inlineSuppression, 0)
	for idx, line := range lines {
		matches := inlineIgnorePattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			out = append(out, inlineSuppression{
				ruleID:  match[1],
				line:    idx + 1,
				expires: match[2],
			})
		}
	}
	return out, nil
}
