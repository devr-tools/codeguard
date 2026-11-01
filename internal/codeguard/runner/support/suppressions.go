package support

import (
	"regexp"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/rules"
)

type inlineSuppression struct {
	ruleID  string
	line    int
	expires string
	// exactLine limits the directive to line itself. codeguard:ignore
	// directives cover the same and the next line; translated gosec
	// justifications cover only the line golangci-lint/gosec would.
	exactLine bool
	// match labels a translated directive in the suppression evidence
	// (core.Suppression.Match); empty for native codeguard:ignore.
	match string
}

var inlineIgnorePattern = regexp.MustCompile(`codeguard:ignore\s+([a-z0-9._*\-]+)(?:\s+until\s+(\d{4}-\d{2}-\d{2}))?`)

// Suppression reasons derived from MatchSuppression, keyed on by the rule-stats
// collector to attribute suppressed findings to their mechanism.
const (
	SuppressionReasonBaseline = "baseline"
	SuppressionReasonWaiver   = "waiver"
	SuppressionReasonInline   = "inline suppression"
	// SuppressionReasonConfidence labels findings removed by the
	// checks.min_confidence policy. It is reported as its own mechanism and is
	// never counted as a suppression.
	SuppressionReasonConfidence = "confidence"
)

func suppressionReason(suppression *core.Suppression) string {
	if suppression == nil {
		return ""
	}
	if suppression.Kind == "inline" {
		return SuppressionReasonInline
	}
	return suppression.Kind
}

// MatchSuppression returns structured evidence of the suppression covering
// finding, checking the baseline (by any of its fingerprints), then waivers,
// then inline directives; it returns nil when none applies.
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
	index := inlineSuppressionsFor(sc, finding.Path)
	if index == nil {
		return nil
	}
	if directive, ok := index.match(sc, finding); ok {
		return &core.Suppression{Kind: "inline", Match: directive.match}
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
		if waiver.Rule != "*" && rules.ResolveRuleAlias(waiver.Rule) != finding.RuleID {
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

func suppressionExpired(expires string, today time.Time) bool {
	if expires == "" {
		return false
	}
	parsed, err := time.Parse("2006-01-02", expires)
	return err == nil && parsed.Before(DateOnly(today))
}
