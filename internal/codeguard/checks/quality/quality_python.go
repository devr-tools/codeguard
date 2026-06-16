package quality

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func pythonFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, fn := range pythonFunctions(string(data)) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	findings = append(findings, pythonAIQualityFindings(env, file, data)...)
	findings = append(findings, pythonPerformanceFindings(env, file, data)...)
	return append(fileLengthFindingWithSignals(env, file, data, findings), findings...)
}

// pythonFunctions extracts function metrics from the structured Python
// parser, so strings or comments that merely look like code are ignored and
// multiline signatures are handled.
func pythonFunctions(source string) []functionMetrics {
	return parsedFunctionMetrics(support.ParsePython(source), pythonComplexity)
}

// maskedFunctionBody joins the masked statements of a function and its
// nested functions, mirroring the full lexical body.
func maskedFunctionBody(fn *support.ParsedFunction) string {
	parts := make([]string, 0, len(fn.Statements))
	for _, statement := range fn.Statements {
		parts = append(parts, statement.Text)
	}
	for _, nested := range fn.Nested {
		parts = append(parts, maskedFunctionBody(nested))
	}
	return strings.Join(parts, "\n")
}

func pythonComplexity(body string) int {
	complexity := 1
	normalized := " " + strings.ReplaceAll(body, "\n", " ") + " "
	for _, pattern := range []string{" if ", " elif ", " for ", " while ", " except ", " case ", " and ", " or "} {
		complexity += strings.Count(normalized, pattern)
	}
	return complexity
}
