package performance

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func cppRebuildCascadeFindings(env support.Context, target core.TargetConfig) []core.Finding {
	if !toggleEnabled(env.Config.Checks.PerformanceRules.DetectRebuildCascade) {
		return nil
	}
	graph := support.BuildCPPDependencyGraph(env, target)
	if graph == nil || len(graph.Graph.Nodes) == 0 {
		return nil
	}
	return dependencyRebuildCascadeFindings(env, graph.Graph, cppRebuildCandidates(env, graph), rebuildCascadeSpec{
		hotRuleID:       "performance.cpp.hot-header",
		amplifierRuleID: "performance.cpp.rebuild-amplifier",
		hotMessage: func(module string, count int, threshold int, sample string) string {
			return fmt.Sprintf("C++ file %q is included or imported by %d target-local files; max is %d, so edits fan out recompilation broadly%s", module, count, threshold, sample)
		},
		amplifierMessage: func(module string, count int, threshold int, sample string) string {
			return fmt.Sprintf("C++ file %q has %d transitive target-local dependents; max is %d, so changes amplify rebuild cascades%s", module, count, threshold, sample)
		},
	})
}

func cppRebuildCandidates(env support.Context, graph *support.CPPDependencyGraph) []string {
	if env.Mode != core.ScanModeDiff {
		return append([]string(nil), graph.Graph.Order...)
	}
	seen := make(map[string]bool)
	modules := make([]string, 0)
	for _, changed := range env.ChangedFiles {
		module, ok := graph.FileToModule[filepath.ToSlash(changed)]
		if !ok || seen[module] {
			continue
		}
		seen[module] = true
		modules = append(modules, module)
	}
	slices.Sort(modules)
	return modules
}
