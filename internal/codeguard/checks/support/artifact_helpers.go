package support

import (
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func ArtifactSafeID(value string) string {
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", "_", "-")
	out := strings.Trim(replacer.Replace(strings.ToLower(strings.TrimSpace(value))), "-")
	if out == "" {
		return "target"
	}
	return out
}

func WeightedFindingComponents(findings []core.Finding, weights map[string]int) ([]core.SlopScoreComponent, int, int, bool) {
	componentCounts := map[string]int{}
	signals := 0
	total := 0
	for _, finding := range findings {
		weight, ok := weights[finding.RuleID]
		if !ok {
			continue
		}
		componentCounts[finding.RuleID]++
		signals++
		total += weight
	}
	if signals == 0 {
		return nil, 0, 0, false
	}
	componentIDs := make([]string, 0, len(componentCounts))
	for ruleID := range componentCounts {
		componentIDs = append(componentIDs, ruleID)
	}
	sort.Strings(componentIDs)
	components := make([]core.SlopScoreComponent, 0, len(componentIDs))
	for _, ruleID := range componentIDs {
		weight := weights[ruleID]
		count := componentCounts[ruleID]
		components = append(components, core.SlopScoreComponent{
			RuleID:       ruleID,
			Count:        count,
			Weight:       weight,
			Contribution: count * weight,
		})
	}
	return components, signals, total, true
}
