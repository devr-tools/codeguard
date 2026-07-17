package design

import (
	"fmt"
	"sort"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const (
	defaultStabilityMinimumFanIn = 3
	defaultMaxInstabilityDelta   = 0.35
)

func graphPolicyFindings(env support.Context, target core.TargetConfig, graph *moduleGraph) []core.Finding {
	if graph == nil {
		return nil
	}
	findings := reachabilityFindings(env, target, graph)
	findings = append(findings, stabilityDirectionFindings(env, graph)...)
	return findings
}

func reachabilityFindings(env support.Context, target core.TargetConfig, graph *moduleGraph) []core.Finding {
	policy := env.Config.Checks.DesignRules.Reachability
	if policy == nil || !designPolicyEnabled(policy.Enabled) {
		return nil
	}
	entrypointPatterns := append([]string(nil), policy.Entrypoints...)
	entrypointPatterns = append(entrypointPatterns, target.Entrypoints...)
	if len(entrypointPatterns) == 0 {
		return nil
	}

	reachable := reachableModulesFromEntrypoints(graph, entrypointPatterns)

	findings := make([]core.Finding, 0)
	for _, module := range graph.sortedOrder() {
		node := graph.modules[module]
		if reachable[module] || designPathMatches(policy.IgnorePaths, node.file) {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "design.unreachable-module",
			Level:   "warn",
			Path:    node.file,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("module %q is not reachable from an approved entrypoint", module),
		}))
	}
	return findings
}

func reachableModulesFromEntrypoints(graph *moduleGraph, entrypointPatterns []string) map[string]bool {
	reachable := make(map[string]bool, len(graph.modules))
	queue := initialReachabilityQueue(graph, entrypointPatterns, reachable)
	for len(queue) > 0 {
		module := queue[0]
		queue = queue[1:]
		for _, edge := range graph.modules[module].edges {
			if reachable[edge.to] {
				continue
			}
			reachable[edge.to] = true
			queue = append(queue, edge.to)
		}
	}
	return reachable
}

func initialReachabilityQueue(graph *moduleGraph, entrypointPatterns []string, reachable map[string]bool) []string {
	queue := make([]string, 0)
	for file, module := range graph.fileToModule {
		if designPathMatches(entrypointPatterns, file) && !reachable[module] {
			reachable[module] = true
			queue = append(queue, module)
		}
	}
	for module := range graph.modules {
		if !reachable[module] && designPathMatches(entrypointPatterns, module) {
			reachable[module] = true
			queue = append(queue, module)
		}
	}
	return queue
}

func stabilityDirectionFindings(env support.Context, graph *moduleGraph) []core.Finding {
	policy := env.Config.Checks.DesignRules.Stability
	if policy == nil || !designPolicyEnabled(policy.Enabled) {
		return nil
	}
	minimumFanIn := policy.MinimumFanIn
	if minimumFanIn <= 0 {
		minimumFanIn = defaultStabilityMinimumFanIn
	}
	maxDelta := policy.MaxInstabilityDelta
	if maxDelta <= 0 {
		maxDelta = defaultMaxInstabilityDelta
	}

	fanOut, fanIn := graph.fanCounts()
	instability := moduleInstability(graph, fanOut, fanIn)
	stability := stabilityCheck{
		graph:       graph,
		policy:      policy,
		instability: instability,
		maxDelta:    maxDelta,
	}

	findings := make([]core.Finding, 0)
	for _, module := range graph.sortedOrder() {
		node := graph.modules[module]
		if fanIn[module] < minimumFanIn || designPathMatches(policy.IgnorePaths, node.file) {
			continue
		}
		edges := append([]moduleGraphEdge(nil), node.edges...)
		sort.Slice(edges, func(i, j int) bool { return edges[i].to < edges[j].to })
		for _, edge := range edges {
			if !stability.violation(module, edge) {
				continue
			}
			findingPath, findingLine := graphImportLocation(graph, module, edge)
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID: "design.stability-direction",
				Level:  "warn",
				Path:   findingPath,
				Line:   findingLine,
				Column: 1,
				Message: fmt.Sprintf("stable module %q (instability %.2f, fan-in %d) depends on less stable module %q (instability %.2f)",
					module, instability[module], fanIn[module], edge.to, instability[edge.to]),
			}))
		}
	}
	return findings
}

func moduleInstability(graph *moduleGraph, fanOut map[string]int, fanIn map[string]int) map[string]float64 {
	instability := make(map[string]float64, len(graph.modules))
	for module := range graph.modules {
		total := fanIn[module] + fanOut[module]
		if total > 0 {
			instability[module] = float64(fanOut[module]) / float64(total)
		}
	}
	return instability
}

type stabilityCheck struct {
	graph       *moduleGraph
	policy      *core.DesignStabilityConfig
	instability map[string]float64
	maxDelta    float64
}

func (check stabilityCheck) violation(module string, edge moduleGraphEdge) bool {
	target := check.graph.modules[edge.to]
	if target == nil || designPathMatches(check.policy.IgnorePaths, target.file) {
		return false
	}
	return check.instability[edge.to]-check.instability[module] > check.maxDelta
}

func graphImportLocation(graph *moduleGraph, from string, edge moduleGraphEdge) (string, int) {
	for _, imported := range graph.imports {
		if imported.from == from && imported.to == edge.to && imported.line == edge.line {
			return imported.sourceFile, positiveImportLine(imported.line)
		}
	}
	return graph.modules[from].file, positiveImportLine(edge.line)
}

func designPolicyEnabled(enabled *bool) bool {
	return enabled == nil || *enabled
}
