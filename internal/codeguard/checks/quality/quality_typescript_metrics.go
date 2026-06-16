package quality

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

// typeScriptFunctions extracts function metrics from the structured C-like
// parser, so functions inside comments or template literals are ignored and
// braces within string literals cannot corrupt body extents.
func typeScriptFunctions(source string) []functionMetrics {
	return parsedFunctionMetrics(support.ParseCLike(source, support.CLikeTypeScript), typeScriptComplexity)
}

func typeScriptComplexity(body string) int {
	complexity := 1
	for _, pattern := range []string{"if (", "for (", "while (", "case ", "catch (", "&&", "||", " ? "} {
		complexity += strings.Count(body, pattern)
	}
	return complexity
}
