package design

import (
	"fmt"
	"path/filepath"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const changeImpactDependentSample = 12

type targetModuleGraph struct {
	target core.TargetConfig
	graph  *moduleGraph
}

// changeImpactFindings computes the transitive impact radius for changed
// modules in diff mode, emits a change-impact report artifact, and warns when
// a changed module exceeds the configured dependent threshold.
func changeImpactFindings(env support.Context, graphs []targetModuleGraph) []core.Finding {
	if !env.DiffMode || !designToggleEnabled(env.Config.Checks.DesignRules.DetectHighImpactChanges) {
		return nil
	}
	threshold := env.Config.Checks.DesignRules.HighImpactChangeThreshold
	if threshold <= 0 {
		threshold = 10
	}
	entries := make([]core.ChangeImpactEntry, 0)
	findings := make([]core.Finding, 0)
	for _, item := range graphs {
		for _, changed := range env.ChangedFiles {
			entry, ok := changeImpactEntry(item, changed)
			if !ok {
				continue
			}
			entries = append(entries, entry)
			if entry.TransitiveDependents > threshold {
				findings = append(findings, highImpactChangeFinding(env, entry, threshold))
			}
		}
	}
	if len(entries) > 0 && env.AddReportArtifact != nil {
		env.AddReportArtifact(core.NewChangeImpactArtifact(env.DiffBaseRef, entries))
	}
	return findings
}

func changeImpactEntry(item targetModuleGraph, changed string) (core.ChangeImpactEntry, bool) {
	module, ok := item.graph.fileToModule[filepath.ToSlash(changed)]
	if !ok {
		return core.ChangeImpactEntry{}, false
	}
	dependents := item.graph.transitiveDependents(module)
	return core.ChangeImpactEntry{
		Target:               item.target.Name,
		Language:             item.graph.language,
		Module:               module,
		File:                 changed,
		TransitiveDependents: len(dependents),
		Dependents:           sampleStrings(dependents, changeImpactDependentSample),
	}, true
}

func highImpactChangeFinding(env support.Context, entry core.ChangeImpactEntry, threshold int) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID: "design.high-impact-change",
		Level:  "warn",
		Path:   entry.File,
		Line:   0,
		Column: 1,
		Message: fmt.Sprintf("changed module %q has %d transitive dependents; max is %d",
			entry.Module, entry.TransitiveDependents, threshold),
	})
}

func sampleStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return append([]string(nil), values...)
	}
	return append([]string(nil), values[:limit]...)
}
