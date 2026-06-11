package quality

import (
	"regexp"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	csharpMethodPattern = regexp.MustCompile(`^\s*(?:\[[^\]]+\]\s*)*(?:(?:public|protected|private|internal|static|virtual|override|sealed|abstract|async|partial|unsafe|extern|new)\s+)+[\w<>\[\],.?&\s]+\s+([A-Za-z_]\w*)\s*\(([^)]*)\)\s*(?:where [^{]+)?\{`)
	rubyFunctionPattern = regexp.MustCompile(`^\s*def\s+(?:self\.)?([A-Za-z_]\w*[!?=]?)\s*(?:\(([^)]*)\)|\s+([^#]+))?`)
	csharpControlWords  = map[string]struct{}{"if": {}, "for": {}, "foreach": {}, "while": {}, "switch": {}, "catch": {}, "return": {}, "new": {}, "throw": {}, "lock": {}, "using": {}}
)

func rustFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := fileLengthFinding(env, file, data)
	for _, fn := range parsedFunctionMetrics(support.ParseRustFunctions(string(data)), rustParameterCount, rustComplexity) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return findings
}

func javaFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := fileLengthFinding(env, file, data)
	for _, fn := range parsedFunctionMetrics(support.ParseJavaFunctions(string(data)), typedParameterCount, braceComplexity) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return findings
}

func csharpFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := fileLengthFinding(env, file, data)
	for _, fn := range braceLanguageFunctions(string(data), csharpMethodPattern, typedParameterCount, braceComplexity, csharpControlWords) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return findings
}

func rubyFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := fileLengthFinding(env, file, data)
	for _, fn := range rubyFunctions(string(data)) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return findings
}
