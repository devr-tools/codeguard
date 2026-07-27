package quality

import "strings"

func typeScriptComplexity(body string) int {
	complexity := 1
	for _, pattern := range []string{"if (", "for (", "while (", "case ", "catch (", "&&", "||", " ? "} {
		complexity += strings.Count(body, pattern)
	}
	return complexity
}
