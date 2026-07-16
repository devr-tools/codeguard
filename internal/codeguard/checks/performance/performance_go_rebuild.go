package performance

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const (
	defaultHotPackageImporterThreshold     = 8
	defaultRebuildAmplifierThreshold       = 20
	rebuildCascadePackageSampleLimit   int = 4
)

func goRebuildCascadeFindings(env support.Context, target core.TargetConfig) []core.Finding {
	if !toggleEnabled(env.Config.Checks.PerformanceRules.DetectRebuildCascade) {
		return nil
	}
	graph := support.BuildGoPackageImportGraph(env, target)
	if graph == nil || len(graph.Graph.Nodes) == 0 {
		return nil
	}
	hotThreshold := env.Config.Checks.PerformanceRules.HotPackageImporterThreshold
	if hotThreshold <= 0 {
		hotThreshold = defaultHotPackageImporterThreshold
	}
	amplifierThreshold := env.Config.Checks.PerformanceRules.RebuildAmplifierThreshold
	if amplifierThreshold <= 0 {
		amplifierThreshold = defaultRebuildAmplifierThreshold
	}

	reverse := support.ReverseDependencyMap(graph.Graph)
	candidates := rebuildCascadeCandidatePackages(env, graph)
	findings := make([]core.Finding, 0)
	for _, pkg := range candidates {
		node, ok := graph.Graph.Nodes[pkg]
		if !ok {
			continue
		}
		importers := append([]string(nil), reverse[pkg]...)
		slices.Sort(importers)
		if len(importers) > hotThreshold {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "performance.go.hot-package",
				Level:   "warn",
				Path:    node.Path,
				Line:    0,
				Column:  1,
				Message: fmt.Sprintf("Go package %q is imported by %d packages; max is %d, so edits here fan out rebuilds broadly%s", pkg, len(importers), hotThreshold, rebuildCascadeSample(importers)),
			}))
		}
		dependents := support.TransitiveDependents(reverse, pkg)
		if len(dependents) > amplifierThreshold {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "performance.go.rebuild-amplifier",
				Level:   "warn",
				Path:    node.Path,
				Line:    0,
				Column:  1,
				Message: fmt.Sprintf("Go package %q has %d transitive dependents; max is %d, so changes here amplify rebuild cascades%s", pkg, len(dependents), amplifierThreshold, rebuildCascadeSample(dependents)),
			}))
		}
	}
	return findings
}

func rebuildCascadeCandidatePackages(env support.Context, graph *support.GoPackageImportGraph) []string {
	if env.Mode != core.ScanModeDiff {
		return append([]string(nil), graph.Graph.Order...)
	}
	seen := make(map[string]bool)
	packages := make([]string, 0)
	for _, changed := range env.ChangedFiles {
		pkg, ok := graph.FileToPackage[filepath.ToSlash(changed)]
		if !ok || seen[pkg] {
			continue
		}
		seen[pkg] = true
		packages = append(packages, pkg)
	}
	slices.Sort(packages)
	return packages
}

func rebuildCascadeSample(values []string) string {
	if len(values) == 0 {
		return ""
	}
	sample := values
	if len(sample) > rebuildCascadePackageSampleLimit {
		sample = sample[:rebuildCascadePackageSampleLimit]
	}
	suffix := ""
	if len(values) > len(sample) {
		suffix = ", ..."
	}
	return fmt.Sprintf(" (sample: %s%s)", strings.Join(sample, ", "), suffix)
}
