package support

import (
	"sort"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// ScriptQuerySpec is the tree-based counterpart of ScriptRegexSpec: a
// compiled query plus a classifier that turns each match into the 1-based
// line to report (or rejects it). Findings carry the same rule ID, level,
// and message as the regex path; Confidence marks how precise the query is
// (migrated rules use core.ConfidenceHigh because the grammar removes the
// regex path's false-positive classes by construction).
type ScriptQuerySpec struct {
	Query      *CompiledQuery
	RuleID     string
	Level      string
	Message    string
	Confidence string
	Classify   func(hit QueryHit) (line int, ok bool)
}

// ScriptQueryFindings evaluates one query-based rule against a parsed tree,
// deduplicating per line exactly like the regex path. The boolean reports
// whether the tree path ran: false means the query failed to compile for
// this grammar and the caller must fall back to ScriptRegexFindings.
func ScriptQueryFindings(env Context, file string, tree *SyntaxTree, spec ScriptQuerySpec) ([]core.Finding, bool) {
	hits, err := tree.Query(spec.Query)
	if err != nil {
		return nil, false
	}
	lines := make([]int, 0, len(hits))
	seen := make(map[int]struct{}, len(hits))
	for _, hit := range hits {
		line, ok := spec.Classify(hit)
		if !ok {
			continue
		}
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	sort.Ints(lines)
	findings := make([]core.Finding, 0, len(lines))
	for _, line := range lines {
		findings = append(findings, env.NewFinding(FindingInput{
			RuleID:     spec.RuleID,
			Level:      spec.Level,
			Path:       file,
			Line:       line,
			Column:     1,
			Message:    spec.Message,
			Confidence: spec.Confidence,
		}))
	}
	return findings, true
}
