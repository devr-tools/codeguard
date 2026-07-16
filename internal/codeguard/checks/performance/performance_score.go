package performance

import (
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// performanceScoreWeights assigns each performance rule a weight by family,
// mirroring aiSlopRuleWeights for the slop_score artifact. Weights order the
// families by typical production impact and are deliberately simple and
// stable so scores stay comparable across scans:
//
//	5 — query-in-loop (N+1): per-item round trips multiply latency directly
//	4 — blocking I/O in request/async paths: stalls a handler or event loop
//	4 — unbounded concurrency: loop-spawned goroutines/promises/tasks
//	3 — memory pressure: unbounded reads, leaked timers/listeners
//	2 — repeated loop work: regex compiles, defers, sleeps, serial awaits
//	1 — allocation churn: string growth and alloc-heavy loop bodies
var performanceScoreWeights = map[string]int{
	"performance.n-plus-one-query": 5,

	"performance.sync-io-in-request-path":       4,
	"performance.typescript.sync-io-in-handler": 4,
	"performance.javascript.sync-io-in-handler": 4,
	"performance.python.sync-io-in-async":       4,

	"performance.unbounded-goroutines-in-loop":     4,
	"performance.typescript.unbounded-concurrency": 4,
	"performance.javascript.unbounded-concurrency": 4,
	"performance.python.unbounded-concurrency":     4,

	"performance.unbounded-read":                 3,
	"performance.go.timer-leak-in-loop":          3,
	"performance.typescript.timer-listener-leak": 3,
	"performance.javascript.timer-listener-leak": 3,

	"performance.regex-compile-in-loop":    2,
	"performance.go.defer-in-loop":         2,
	"performance.go.sleep-in-loop":         2,
	"performance.typescript.await-in-loop": 2,
	"performance.javascript.await-in-loop": 2,

	"performance.go.alloc-in-loop":      1,
	"performance.string-concat-in-loop": 1,
}

// maybePutPerformanceScoreArtifact publishes the per-target performance_score
// artifact when the section produced findings, mirroring
// maybePutAISlopArtifact in checks/quality.
func maybePutPerformanceScoreArtifact(env support.Context, target core.TargetConfig, findings []core.Finding) {
	if env.PutArtifact == nil {
		return
	}
	artifact, ok := performanceScoreArtifact(target, findings)
	if !ok {
		return
	}
	recordPerformanceScoreHistory(env, &artifact)
	env.PutArtifact(artifact)
}

// performanceScoreArtifact computes the weighted score: each finding
// contributes its rule's family weight, and the total saturates at 100 via
// the same min(10*sum, 100) scaling the slop score uses.
func performanceScoreArtifact(target core.TargetConfig, findings []core.Finding) (core.Artifact, bool) {
	componentCounts := map[string]int{}
	signals := 0
	total := 0
	for _, finding := range findings {
		weight, ok := performanceScoreWeights[finding.RuleID]
		if !ok {
			continue
		}
		componentCounts[finding.RuleID]++
		signals++
		total += weight
	}
	if signals == 0 {
		return core.Artifact{}, false
	}
	componentIDs := make([]string, 0, len(componentCounts))
	for ruleID := range componentCounts {
		componentIDs = append(componentIDs, ruleID)
	}
	sort.Strings(componentIDs)
	components := make([]core.SlopScoreComponent, 0, len(componentIDs))
	for _, ruleID := range componentIDs {
		weight := performanceScoreWeights[ruleID]
		count := componentCounts[ruleID]
		components = append(components, core.SlopScoreComponent{
			RuleID:       ruleID,
			Count:        count,
			Weight:       weight,
			Contribution: count * weight,
		})
	}
	language := support.NormalizedLanguage(target.Language)
	if language == "" {
		language = "go"
	}
	score := total * 10
	if score > 100 {
		score = 100
	}
	return support.NewPerformanceScoreArtifact(
		"performance_score."+language+"."+performanceArtifactSafeID(target.Name),
		language,
		target.Path,
		core.PerformanceScoreArtifact{
			Score:      score,
			Signals:    signals,
			Components: components,
		},
	), true
}

// performanceArtifactSafeID mirrors quality.artifactSafeID for artifact ID
// segments derived from target names.
func performanceArtifactSafeID(value string) string {
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", "_", "-")
	out := strings.Trim(replacer.Replace(strings.ToLower(strings.TrimSpace(value))), "-")
	if out == "" {
		return "target"
	}
	return out
}
