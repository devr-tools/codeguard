package quality

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func pythonFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := fileLengthFinding(env, file, data)
	for _, fn := range parsedFunctionMetrics(support.ParsePythonFunctions(string(data)), pythonParameterCount, pythonComplexity) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return findings
}

func pythonComplexity(body string) int {
	complexity := 1
	for _, pattern := range []string{" if ", " elif ", " for ", " while ", " except ", " case ", " and ", " or "} {
		complexity += strings.Count(" "+body+" ", pattern)
	}
	return complexity
}

func pythonParameterCount(signature string) int {
	count := 0
	for _, part := range splitTopLevelDelimited(signature) {
		if part == "*" || part == "/" {
			continue
		}
		count++
	}
	return count
}
