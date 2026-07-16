package support

import (
	"fmt"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type conanLiteral struct {
	value string
	line  int
}

func parseConanPythonManifest(root string, rel string, data []byte) core.SupplyChainManifest {
	manifest := core.SupplyChainManifest{
		Ecosystem:      "conan",
		PackageManager: "conan",
		Path:           rel,
		Lockfiles:      presentLockfiles(root, rel, []string{"conan.lock"}),
	}
	tokens := scanPythonTokens(string(data))
	constants := make(map[string][]conanLiteral)
	for idx := 0; idx < len(tokens); {
		if values, name, scope, startLine, next, assignment := parseConanAssignment(tokens, idx, constants); assignment {
			if values != nil {
				constants[name] = values
				if scope != "" {
					appendConanLiterals(&manifest, values, scope, startLine)
				}
			} else if scope != "" {
				manifest.AnalysisLimitations = append(manifest.AnalysisLimitations,
					fmt.Sprintf("conanfile.py %s declaration at line %d is dynamic; CodeGuard did not execute Python to resolve it", name, startLine))
			}
			idx = next
			continue
		}
		if values, scope, startLine, next, call := parseConanRequiresCall(tokens, idx, constants); call {
			if values != nil {
				appendConanLiterals(&manifest, values, scope, startLine)
			} else {
				manifest.AnalysisLimitations = append(manifest.AnalysisLimitations,
					fmt.Sprintf("conanfile.py self.%s() call at line %d is dynamic; CodeGuard did not execute Python to resolve it", scopeMethodName(scope), startLine))
			}
			idx = next
			continue
		}
		idx++
	}
	sortDependencies(manifest.Dependencies)
	manifest.AnalysisLimitations = uniqueSortedStrings(manifest.AnalysisLimitations)
	return manifest
}

func parseConanAssignment(tokens []pythonToken, idx int, constants map[string][]conanLiteral) ([]conanLiteral, string, string, int, int, bool) {
	if idx+1 >= len(tokens) || tokens[idx].kind != 'i' || tokens[idx+1].value != "=" {
		return nil, "", "", 0, idx + 1, false
	}
	name := tokens[idx].value
	// Class-level Conan declarations and constants are normally at module or
	// class indentation. Avoid treating method-local variables as manifests.
	if tokens[idx].column > 5 {
		return nil, "", "", 0, idx + 1, false
	}
	end := pythonExpressionEnd(tokens, idx+2)
	values, ok := evaluateConanLiteralExpression(tokens[idx+2:end], constants)
	scope := ""
	switch name {
	case "requires":
		scope = "runtime"
	case "tool_requires":
		scope = "build"
	}
	if !ok {
		return nil, name, scope, tokens[idx].line, max(end, idx+2), true
	}
	return values, name, scope, tokens[idx].line, max(end, idx+2), true
}

func parseConanRequiresCall(tokens []pythonToken, idx int, constants map[string][]conanLiteral) ([]conanLiteral, string, int, int, bool) {
	if idx+4 >= len(tokens) || tokens[idx].value != "self" || tokens[idx+1].value != "." || tokens[idx+2].kind != 'i' || tokens[idx+3].value != "(" {
		return nil, "", 0, idx + 1, false
	}
	method := tokens[idx+2].value
	scope := ""
	switch method {
	case "requires":
		scope = "runtime"
	case "tool_requires":
		scope = "build"
	default:
		return nil, "", 0, idx + 1, false
	}
	closeIdx := matchingPythonDelimiter(tokens, idx+3)
	if closeIdx < 0 {
		return nil, scope, tokens[idx].line, len(tokens), true
	}
	arguments := tokens[idx+4 : closeIdx]
	firstEnd := firstPythonCallArgumentEnd(arguments)
	values, ok := evaluateConanLiteralExpression(arguments[:firstEnd], constants)
	if !ok || len(values) != 1 {
		return nil, scope, tokens[idx].line, closeIdx + 1, true
	}
	return values, scope, tokens[idx].line, closeIdx + 1, true
}

func appendConanLiterals(manifest *core.SupplyChainManifest, values []conanLiteral, scope string, declarationLine int) {
	section := "requires"
	if scope == "build" {
		section = "tool_requires"
	}
	for _, literal := range values {
		line := literal.line
		if line == 0 {
			line = declarationLine
		}
		if dep, ok := parseConanReference(literal.value, section, line); ok {
			manifest.Dependencies = append(manifest.Dependencies, dep)
		}
	}
}

func scopeMethodName(scope string) string {
	if scope == "build" {
		return "tool_requires"
	}
	return "requires"
}
