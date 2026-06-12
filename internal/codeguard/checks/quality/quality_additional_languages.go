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
	findings := make([]core.Finding, 0)
	for _, fn := range clikeQualityFunctions(string(data), support.CLikeRust, rustComplexity) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return append(fileLengthFindingWithSignals(env, file, data, findings), findings...)
}

func javaFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, fn := range clikeQualityFunctions(string(data), support.CLikeJava, braceComplexity) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return append(fileLengthFindingWithSignals(env, file, data, findings), findings...)
}

// clikeQualityFunctions extracts function metrics from the structured C-like
// parser, so comments and string literals cannot produce phantom functions
// or corrupt brace matching.
func clikeQualityFunctions(source string, lang support.CLikeLanguage, complexityFn func(string) int) []functionMetrics {
	return parsedFunctionMetrics(support.ParseCLike(source, lang), complexityFn)
}

func csharpFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, fn := range braceLanguageFunctions(string(data), csharpMethodPattern, typedParameterCount, braceComplexity, csharpControlWords) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return append(fileLengthFindingWithSignals(env, file, data, findings), findings...)
}

func rubyFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, fn := range rubyFunctions(string(data)) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return append(fileLengthFindingWithSignals(env, file, data, findings), findings...)
}
