package design

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func pythonTargetFindings(env support.Context, target core.TargetConfig, graph pythonImportGraph) []core.Finding {
	findings := make([]core.Finding, 0, len(graph.graph.Order))
	findings = append(findings, pythonStructuralFindings(env, target)...)
	for _, module := range graph.graph.Order {
		node := graph.graph.Nodes[module]
		findings = append(findings, genericPythonModuleNameFindings(env, node.Path)...)
		findings = append(findings, directPythonBoundaryFindings(env, node, graph.entrypoints)...)
	}
	findings = append(findings, transitivePythonEntrypointFindings(env, graph)...)
	findings = append(findings, pythonImportCycleFindings(env, graph)...)
	return findings
}

func genericPythonModuleNameFindings(env support.Context, file string) []core.Finding {
	moduleName := strings.ToLower(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)))
	for _, forbidden := range env.Config.Checks.DesignRules.ForbiddenPackageNames {
		if strings.EqualFold(moduleName, forbidden) {
			return []core.Finding{env.NewFinding(support.FindingInput{
				RuleID:  "design.python.generic-module-name",
				Level:   "warn",
				Path:    file,
				Line:    1,
				Column:  1,
				Message: fmt.Sprintf("module name %q is too generic", moduleName),
			})}
		}
	}
	return nil
}

func isPublicPythonModule(file string, target core.TargetConfig) bool {
	slash := filepath.ToSlash(file)
	base := filepath.Base(slash)
	if strings.HasPrefix(base, "_") || strings.HasPrefix(slash, "tests/") || strings.Contains(slash, "/tests/") {
		return false
	}
	for _, entrypoint := range target.Entrypoints {
		if filepath.ToSlash(entrypoint) == slash {
			return false
		}
	}
	return true
}

func pythonEntrypointModules(paths []string) map[string]struct{} {
	modules := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		module := pythonModuleName(path)
		if module != "" {
			modules[module] = struct{}{}
		}
	}
	return modules
}

func pythonModuleName(path string) string {
	slash := filepath.ToSlash(path)
	slash = strings.TrimSuffix(slash, ".py")
	slash = strings.TrimSuffix(slash, "/__init__")
	slash = strings.TrimPrefix(slash, "./")
	return strings.ReplaceAll(slash, "/", ".")
}

func pythonPackageName(path string) string {
	slash := filepath.ToSlash(path)
	module := pythonModuleName(path)
	if strings.HasSuffix(slash, "/__init__.py") {
		return module
	}
	if cut := strings.LastIndex(module, "."); cut >= 0 {
		return module[:cut]
	}
	return ""
}
