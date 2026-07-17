package design

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// architectureBoundaryFindings enforces the configured, language-neutral
// architecture boundaries over a target's source-level import graph.
func architectureBoundaryFindings(env support.Context, target core.TargetConfig, graph *moduleGraph) []core.Finding {
	_ = target
	if graph == nil {
		return nil
	}
	rules := env.Config.Checks.DesignRules
	findings := make([]core.Finding, 0, len(graph.imports))
	findings = append(findings, unassignedModuleFindings(env, graph, rules)...)
	findings = append(findings, layerBoundaryFindings(env, graph, rules.Layers)...)
	findings = append(findings, domainBoundaryFindings(env, graph, rules.Domains)...)
	findings = append(findings, capabilityBoundaryFindings(env, graph, rules.Capabilities)...)
	return deduplicateArchitectureFindings(findings)
}

func unassignedModuleFindings(env support.Context, graph *moduleGraph, rules core.DesignRulesConfig) []core.Finding {
	if rules.RequireBoundaryAssignment == nil || !*rules.RequireBoundaryAssignment || (len(rules.Layers) == 0 && len(rules.Domains) == 0) {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, module := range graph.sortedOrder() {
		node := graph.modules[module]
		missing := make([]string, 0, 2)
		if len(rules.Layers) > 0 && layerForPath(rules.Layers, node.file) == nil {
			missing = append(missing, "layer")
		}
		if len(rules.Domains) > 0 && domainForPath(rules.Domains, node.file) == nil {
			missing = append(missing, "domain")
		}
		if len(missing) == 0 {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID: "design.unassigned-module", Level: "fail", Path: node.file, Line: 1, Column: 1,
			Message: fmt.Sprintf("module %q is not assigned to a configured %s; add its path to the appropriate design boundary", module, strings.Join(missing, " and ")),
		}))
	}
	return findings
}

func layerBoundaryFindings(env support.Context, graph *moduleGraph, layers []core.DesignLayerConfig) []core.Finding {
	if len(layers) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, imported := range graph.imports {
		sourceLayer := layerForPath(layers, imported.sourceFile)
		if sourceLayer == nil {
			continue
		}
		targetNode, local := graph.modules[imported.to]
		if !local {
			if designPathMatches(sourceLayer.DeniedExternal, imported.specifier) {
				findings = append(findings, env.NewFinding(support.FindingInput{
					RuleID: "design.layer-boundary", Level: "fail", Path: imported.sourceFile, Line: imported.line, Column: 1,
					Message: fmt.Sprintf("layer %q may not import external dependency %q; move the import behind an allowed adapter or remove it", sourceLayer.Name, imported.specifier),
				}))
			}
			continue
		}
		targetLayer := layerForPath(layers, targetNode.file)
		if targetLayer == nil || strings.EqualFold(sourceLayer.Name, targetLayer.Name) {
			continue
		}
		denied := nameInList(sourceLayer.DenyDependOn, targetLayer.Name)
		notAllowed := len(sourceLayer.MayDependOn) > 0 && !nameInList(sourceLayer.MayDependOn, targetLayer.Name)
		if !denied && !notAllowed {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID: "design.layer-boundary", Level: "fail", Path: imported.sourceFile, Line: imported.line, Column: 1,
			Message: fmt.Sprintf("layer %q may not depend on layer %q via %q; invert the dependency or route it through an allowed layer", sourceLayer.Name, targetLayer.Name, imported.specifier),
		}))
	}
	return findings
}

func domainBoundaryFindings(env support.Context, graph *moduleGraph, domains []core.DesignDomainConfig) []core.Finding {
	if len(domains) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, imported := range graph.imports {
		targetNode, local := graph.modules[imported.to]
		if !local {
			continue
		}
		sourceDomain := domainForPath(domains, imported.sourceFile)
		targetDomain := domainForPath(domains, targetNode.file)
		if sourceDomain == nil || targetDomain == nil || strings.EqualFold(sourceDomain.Name, targetDomain.Name) {
			continue
		}
		if designPathMatches(targetDomain.DataPaths, targetNode.file) {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID: "design.data-ownership", Level: "fail", Path: imported.sourceFile, Line: imported.line, Column: 1,
				Message: fmt.Sprintf("domain %q imports data-owned module %q from domain %q; access that data through the owning domain's public contract", sourceDomain.Name, targetNode.file, targetDomain.Name),
			}))
		}
		allowedDomain := nameInList(sourceDomain.MayDependOn, targetDomain.Name)
		publicTarget := designPathMatches(targetDomain.PublicPaths, targetNode.file)
		if allowedDomain && publicTarget {
			continue
		}
		reason := fmt.Sprintf("domain %q is not in may_depend_on", targetDomain.Name)
		if allowedDomain {
			reason = fmt.Sprintf("target %q is not one of domain %q's public_paths", targetNode.file, targetDomain.Name)
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID: "design.domain-boundary", Level: "fail", Path: imported.sourceFile, Line: imported.line, Column: 1,
			Message: fmt.Sprintf("domain %q may not import %q: %s; use an allowed public domain contract", sourceDomain.Name, imported.specifier, reason),
		}))
	}
	return findings
}

func capabilityBoundaryFindings(env support.Context, graph *moduleGraph, capabilities []core.DesignCapabilityConfig) []core.Finding {
	if len(capabilities) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, imported := range graph.imports {
		candidates := []string{
			imported.specifier,
			strings.ReplaceAll(imported.specifier, "::", "/"),
			imported.to,
		}
		if targetNode, ok := graph.modules[imported.to]; ok {
			candidates = append(candidates, targetNode.file)
		}
		for _, capability := range capabilities {
			if !matchesAnyCandidate(capability.Imports, candidates) || designPathMatches(capability.AllowedPaths, imported.sourceFile) {
				continue
			}
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID: "design.capability-boundary", Level: "fail", Path: imported.sourceFile, Line: imported.line, Column: 1,
				Message: fmt.Sprintf("capability %q is not allowed from %q (import %q); move this access into one of the capability's allowed_paths", capability.Name, imported.sourceFile, imported.specifier),
			}))
		}
	}
	return findings
}

func layerForPath(layers []core.DesignLayerConfig, file string) *core.DesignLayerConfig {
	for idx := range layers {
		if designPathMatches(layers[idx].Paths, file) {
			return &layers[idx]
		}
	}
	return nil
}

func domainForPath(domains []core.DesignDomainConfig, file string) *core.DesignDomainConfig {
	for idx := range domains {
		if designPathMatches(domains[idx].Paths, file) {
			return &domains[idx]
		}
	}
	return nil
}

func matchesAnyCandidate(patterns []string, candidates []string) bool {
	for _, candidate := range candidates {
		if candidate != "" && designPathMatches(patterns, candidate) {
			return true
		}
	}
	return false
}

func nameInList(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

func deduplicateArchitectureFindings(findings []core.Finding) []core.Finding {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].RuleID < findings[j].RuleID
	})
	seen := make(map[string]bool, len(findings))
	result := make([]core.Finding, 0, len(findings))
	for _, finding := range findings {
		key := fmt.Sprintf("%s:%d:%s:%s", finding.Path, finding.Line, finding.RuleID, finding.Message)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, finding)
	}
	return result
}
