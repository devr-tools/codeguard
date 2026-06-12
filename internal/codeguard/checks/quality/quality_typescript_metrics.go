package quality

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

// typeScriptFunctions extracts function metrics from the structured C-like
// parser, so functions inside comments or template literals are ignored and
// braces within string literals cannot corrupt body extents.
func typeScriptFunctions(source string) []functionMetrics {
	file := support.ParseCLike(source, support.CLikeTypeScript)
	functions := make([]functionMetrics, 0)
	for _, fn := range file.AllFunctions() {
		functions = append(functions, functionMetrics{
			Name:       fn.Name,
			StartLine:  fn.StartLine,
			Length:     fn.LineCount(),
			Params:     len(fn.Params),
			Complexity: typeScriptComplexity(maskedFunctionBody(fn)),
		})
	}
	return functions
}

func typeScriptComplexity(body string) int {
	complexity := 1
	for _, pattern := range []string{"if (", "for (", "while (", "case ", "catch (", "&&", "||", " ? "} {
		complexity += strings.Count(body, pattern)
	}
	return complexity
}
