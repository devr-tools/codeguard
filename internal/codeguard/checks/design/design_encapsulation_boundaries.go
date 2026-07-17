package design

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// encapsulationBoundaryFindings applies the path-based public-surface and
// production/test policies to source-level imports from every supported
// language graph.
func encapsulationBoundaryFindings(env support.Context, _ core.TargetConfig, graph *moduleGraph) []core.Finding {
	if graph == nil {
		return nil
	}
	findings := publicSurfaceFindings(env, graph)
	findings = append(findings, productionTestFindings(env, graph)...)
	return findings
}

func publicSurfaceFindings(env support.Context, graph *moduleGraph) []core.Finding {
	surfaces := env.Config.Checks.DesignRules.PublicSurfaces
	if len(surfaces) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, imported := range graph.imports {
		targetPath, ok := resolvedImportPath(graph, imported)
		if !ok {
			continue
		}
		for _, surface := range surfaces {
			if !designPathMatches(surface.Paths, targetPath) ||
				designPathMatches(surface.Paths, imported.sourceFile) ||
				designPathMatches(surface.Entrypoints, targetPath) {
				continue
			}
			name := strings.TrimSpace(surface.Name)
			if name == "" {
				name = strings.Join(surface.Paths, ", ")
			}
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.private-module-import",
				Level:   "fail",
				Path:    imported.sourceFile,
				Line:    positiveImportLine(imported.line),
				Column:  1,
				Message: fmt.Sprintf("module %q imports private module %q from public surface %q; import an approved entrypoint instead", imported.sourceFile, targetPath, name),
			}))
		}
	}
	return findings
}

func productionTestFindings(env support.Context, graph *moduleGraph) []core.Finding {
	policy := env.Config.Checks.DesignRules.ProductionTest
	if policy == nil || !designToggleEnabled(policy.Enabled) {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, imported := range graph.imports {
		if !designPathMatches(policy.ProductionPaths, imported.sourceFile) {
			continue
		}
		targetPath, ok := resolvedImportPath(graph, imported)
		if !ok || !designPathMatches(policy.TestPaths, targetPath) {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "design.production-imports-test",
			Level:   "fail",
			Path:    imported.sourceFile,
			Line:    positiveImportLine(imported.line),
			Column:  1,
			Message: fmt.Sprintf("production module %q imports test-only module %q", imported.sourceFile, targetPath),
		}))
	}
	return findings
}

func resolvedImportPath(graph *moduleGraph, imported moduleGraphImport) (string, bool) {
	if imported.to == "" {
		return "", false
	}
	node, ok := graph.modules[imported.to]
	if !ok || node.file == "" {
		return "", false
	}
	return normalizeDesignPath(node.file), true
}

func positiveImportLine(line int) int {
	if line > 0 {
		return line
	}
	return 1
}
