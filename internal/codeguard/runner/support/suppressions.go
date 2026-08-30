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
	suppression := MatchSuppression(sc, finding)
	return suppression != nil, suppressionReason(suppression)
}

func suppressionReason(suppression *core.Suppression) string {
	if suppression == nil {
		return ""
	}
	if suppression.Kind == "inline" {
		return SuppressionReasonInline
	}
	return suppression.Kind
}

// MatchSuppression returns structured suppression evidence while retaining the
// exact precedence and many-to-many fingerprint semantics of IsSuppressed.
func MatchSuppression(sc Context, finding core.Finding) *core.Suppression {
	if sc.Baseline != nil {
		if entry, ok := sc.Baseline[finding.Fingerprint]; ok {
			return &core.Suppression{Kind: SuppressionReasonBaseline, Match: "exact", BaselineFingerprint: entry.Fingerprint}
		}
		preV2Exact := preV2ExactFingerprint(finding.RuleID, finding.Path, finding.Line, finding.Message)
		if entry, ok := sc.Baseline[preV2Exact]; ok {
			return &core.Suppression{Kind: SuppressionReasonBaseline, Match: "exact", BaselineFingerprint: entry.Fingerprint}
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
			if entry, ok := sc.Baseline[finding.ContextFingerprint]; ok {
				return &core.Suppression{Kind: SuppressionReasonBaseline, Match: "context", BaselineFingerprint: entry.Fingerprint}
			}
		}
		if finding.ContentFingerprint != "" {
			if entry, ok := sc.Baseline[finding.ContentFingerprint]; ok {
				return &core.Suppression{Kind: SuppressionReasonBaseline, Match: "content", BaselineFingerprint: entry.Fingerprint}
			}
		}
	}
	if len(MatchingWaivers(sc, finding)) > 0 {
		return &core.Suppression{Kind: SuppressionReasonWaiver}
	}
	fullPath := findingFullPath(sc, finding.Path)
	if fullPath == "" {
		return nil
	}
	directives, err := parseInlineSuppressions(fullPath)
	if err != nil {
		return nil
	}
	if inlineSuppressionMatches(sc, finding, directives) {
		return &core.Suppression{Kind: "inline"}
	}
	return nil
}

type WaiverMatch struct {
	Index  int
	Waiver core.WaiverConfig
}

func MatchingWaivers(sc Context, finding core.Finding) []WaiverMatch {
	matches := make([]WaiverMatch, 0, 1)
	for idx, waiver := range sc.Cfg.Waivers {
		if waiver.Rule != "*" && waiver.Rule != finding.RuleID {
			continue
		}
		if waiver.Path != "" && !MatchPattern(waiver.Path, finding.Path) {
			continue
		}
		if suppressionExpired(waiver.ExpiresOn, sc.Today) {
			continue
		}
		matches = append(matches, WaiverMatch{Index: idx, Waiver: waiver})
	}
	return matches
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
