package performance

import (
	"slices"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type rebuildCascadeSpec struct {
	hotRuleID        string
	amplifierRuleID  string
	hotMessage       func(id string, count int, threshold int, sample string) string
	amplifierMessage func(id string, count int, threshold int, sample string) string
}

func dependencyRebuildCascadeFindings(env support.Context, graph support.DependencyGraph, candidates []string, spec rebuildCascadeSpec) []core.Finding {
	hotThreshold := env.Config.Checks.PerformanceRules.HotPackageImporterThreshold
	if hotThreshold <= 0 {
		hotThreshold = defaultHotPackageImporterThreshold
	}
	amplifierThreshold := env.Config.Checks.PerformanceRules.RebuildAmplifierThreshold
	if amplifierThreshold <= 0 {
		amplifierThreshold = defaultRebuildAmplifierThreshold
	}
	reverse := support.ReverseDependencyMap(graph)
	findings := make([]core.Finding, 0)
	for _, id := range candidates {
		node, ok := graph.Nodes[id]
		if !ok {
			continue
		}
		importers := append([]string(nil), reverse[id]...)
		slices.Sort(importers)
		if len(importers) > hotThreshold {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID: spec.hotRuleID, Level: "warn", Path: node.Path, Line: 0, Column: 1,
				Message: spec.hotMessage(id, len(importers), hotThreshold, rebuildCascadeSample(importers)),
			}))
		}
		dependents := support.TransitiveDependents(reverse, id)
		if len(dependents) > amplifierThreshold {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID: spec.amplifierRuleID, Level: "warn", Path: node.Path, Line: 0, Column: 1,
				Message: spec.amplifierMessage(id, len(dependents), amplifierThreshold, rebuildCascadeSample(dependents)),
			}))
		}
	}
	return findings
}
