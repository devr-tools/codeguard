package quality

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func pythonFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := fileLengthFinding(env, file, data)
	for _, fn := range pythonFunctions(string(data)) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return findings
}

// pythonFunctions extracts function metrics from the structured Python
// parser, so strings or comments that merely look like code are ignored and
// multiline signatures are handled.
func pythonFunctions(source string) []functionMetrics {
	file := support.ParsePython(source)
	functions := make([]functionMetrics, 0)
	for _, fn := range file.AllFunctions() {
		functions = append(functions, functionMetrics{
			Name:       fn.Name,
			StartLine:  fn.StartLine,
			Length:     fn.LineCount(),
			Params:     len(fn.Params),
			Complexity: pythonComplexity(maskedFunctionBody(fn)),
		})
	}
	return functions
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
