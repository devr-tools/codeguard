package design

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func designToggleEnabled(value *bool) bool {
	return value == nil || *value
}

func graphCycleRuleID(language string, file string) string {
	switch language {
	case "typescript":
		return support.RuleIDForScript(file, "design.typescript.import-cycle", "design.javascript.import-cycle")
	case "rust":
		return "design.rust.import-cycle"
	case "java":
		return "design.java.import-cycle"
	case "cpp":
		return "design.cpp.import-cycle"
	default:
		return ""
	}
}

// importCycleFindings reports one failure per strongly connected component
// with more than one module (or a module importing itself).
func importCycleFindings(env support.Context, graph *moduleGraph) []core.Finding {
	if graph == nil || !designToggleEnabled(env.Config.Checks.DesignRules.DetectImportCycles) {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, component := range graph.stronglyConnectedComponents() {
		if len(component) == 1 && !graphHasSelfEdge(graph, component[0]) {
			continue
		}
		sort.Strings(component)
		node := graph.modules[component[0]]
		if strings.Contains(strings.ToLower(node.file), "/integrations/") {
			continue
		}
		ruleID := graphCycleRuleID(graph.language, node.file)
		if ruleID == "" {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  ruleID,
			Level:   "fail",
			Path:    node.file,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("%s module import cycle detected: %s", graph.language, strings.Join(component, " <-> ")),
		}))
	}
	return findings
}

func graphHasSelfEdge(graph *moduleGraph, module string) bool {
	for _, edge := range graph.modules[module].edges {
		if edge.to == module {
			return true
		}
	}
	return false
}

// godModuleFindings warns when a module's combined fan-in and fan-out exceeds
// the configured threshold.
func godModuleFindings(env support.Context, graph *moduleGraph) []core.Finding {
	if graph == nil || !designToggleEnabled(env.Config.Checks.DesignRules.DetectGodModules) {
		return nil
	}
	threshold := env.Config.Checks.DesignRules.GodModuleThreshold
	if threshold <= 0 {
		threshold = 25
	}
	fanOut, fanIn := graph.fanCounts()
	findings := make([]core.Finding, 0)
	for _, module := range graph.sortedOrder() {
		total := fanOut[module] + fanIn[module]
		if total <= threshold {
			continue
		}
		node := graph.modules[module]
		if allowedCentralDataClientModule(node.file) || allowedCentralUtilityModule(node.file, fanIn[module], fanOut[module]) {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "design.god-module",
			Level:   "warn",
			Path:    node.file,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("module %q has fan-in %d and fan-out %d (total %d); max is %d", module, fanIn[module], fanOut[module], total, threshold),
		}))
	}
	return findings
}

func allowedCentralUtilityModule(file string, fanIn int, fanOut int) bool {
	if fanIn <= 0 || fanOut > 4 {
		return false
	}
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	base := normalized
	if slash := strings.LastIndex(base, "/"); slash >= 0 {
		base = base[slash+1:]
	}
	return containsAnyLocal(base, []string{
		"route-auth", "auth", "authorize", "validate-request", "validation", "validator",
	})
}

func containsAnyLocal(source string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(source, needle) {
			return true
		}
	}
	return false
}

func allowedCentralDataClientModule(file string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	return normalized == "packages/db/src/client.ts" ||
		normalized == "packages/database/src/client.ts" ||
		normalized == "src/db/client.ts" ||
		normalized == "src/database/client.ts" ||
		strings.HasSuffix(normalized, "/packages/db/src/client.ts") ||
		strings.HasSuffix(normalized, "/packages/database/src/client.ts") ||
		strings.HasSuffix(normalized, "/src/db/client.ts") ||
		strings.HasSuffix(normalized, "/src/database/client.ts")
}
