package quality

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	rawLogPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\bfmt\.Print(?:f|ln)?\s*\(`),
		regexp.MustCompile(`\blog\.Print(?:f|ln)?\s*\(`),
		regexp.MustCompile(`\bconsole\.(?:log|warn|error)\s*\(`),
		regexp.MustCompile(`\bprint\s*\(`),
		regexp.MustCompile(`\bstd::cout\b|\bprintf\s*\(`),
	}
	errorLogPattern         = regexp.MustCompile(`(?i)\b(?:logger|log|logging|console|slog|zap)\.\w*(?:error|err|exception|fatal)\w*\s*\(`)
	metricLabelPattern      = regexp.MustCompile(`(?i)\b(?:label|labels|withlabelvalues|withlabels|tags|attributes?)\b`)
	healthReturnOKPattern   = regexp.MustCompile(`(?i)\b(?:ok|healthy|pong|200|http\.statusok|statusok)\b`)
	dependencyPattern       = regexp.MustCompile(`(?i)\b(?:db|database|sql|redis|cache|kafka|queue|http\.client|requests\.|fetch\(|axios\.|grpc|s3|pubsub)\b`)
	logAndIgnoreNextPattern = regexp.MustCompile(`(?i)\b(?:return\s+nil|return\s+None|return\s*;|continue|pass|//\s*ignore|#\s*ignore)\b`)
)

func RunObservability(ctx context.Context, env support.Context) core.SectionResult {
	return support.RunTargetSection(ctx, env, "observability", "Observability", observabilityTargetFindings)
}

func observabilityTargetFindings(_ context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return env.ScanTargetFiles(target, "observability", isObservableSourceFile, func(file string, data []byte) []core.Finding {
		return findingsForFile(env, file, data)
	})
}

func findingsForFile(env support.Context, file string, data []byte) []core.Finding {
	rules := env.Config.Checks.ObservabilityRules
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	code := maskStringsForStructure(file, source)
	lines := strings.Split(source, "\n")
	codeLines := strings.Split(code, "\n")
	findings := make([]core.Finding, 0)
	hasDependency := dependencyPattern.MatchString(code)
	hasInstrumentation := hasInstrumentationEvidence(source, rules)
	healthRegionLine := 0

	for idx, rawLine := range lines {
		lineNo := idx + 1
		codeLine := ""
		if idx < len(codeLines) {
			codeLine = codeLines[idx]
		}
		trimmed := strings.TrimSpace(codeLine)
		if trimmed == "" || isCommentLine(trimmed) || isTestPath(file) {
			continue
		}
		if enabled(rules.DetectUnstructuredLog) && isRawLogLine(codeLine) && !hasStructuredContext(rawLine) {
			findings = append(findings, newFinding(env, "observability.unstructured-log", "warn", file, lineNo, "raw log call has no structured field context", "medium", "log_kind", "raw"))
		}
		if enabled(rules.DetectErrorWithoutContext) && errorLogPattern.MatchString(codeLine) && !hasErrorContext(rawLine, rules) {
			findings = append(findings, newFinding(env, "observability.error-without-context", "warn", file, lineNo, "error log lacks operation, request, or safe resource context", "medium", "log_kind", "error"))
		}
		if enabled(rules.DetectSensitiveLogData) && isLogLikeLine(codeLine) {
			if token, ok := firstPatternEvidence(rawLine, rules.SensitiveNamePatterns); ok {
				findings = append(findings, newFinding(env, "observability.sensitive-log-data", "fail", file, lineNo, "log call includes sensitive-name evidence", "high", "sensitive_name", token))
			}
		}
		if enabled(rules.DetectHighCardinalityLabel) && metricLabelPattern.MatchString(codeLine) {
			if token, ok := firstPatternEvidence(rawLine, rules.HighCardinalityLabelPatterns); ok {
				findings = append(findings, newFinding(env, "observability.high-cardinality-label", "warn", file, lineNo, "metric label appears to use a high-cardinality value", "high", "label_kind", token))
			}
		}
		if enabled(rules.DetectLogAndIgnore) && errorLogPattern.MatchString(codeLine) && nextFewLinesIgnoreError(codeLines, idx) {
			findings = append(findings, newFinding(env, "observability.log-and-ignore", "warn", file, lineNo, "failure is logged and then ignored or reported as success", "high", "failure_handling", "log-and-ignore"))
		}
		if isHealthPathOrLine(file, codeLine, rules) {
			healthRegionLine = lineNo
		}
		if enabled(rules.DetectShallowHealthCheck) && healthRegionLine > 0 && lineNo <= healthRegionLine+8 && healthReturnOKPattern.MatchString(codeLine) && hasDependency {
			findings = append(findings, newFinding(env, "observability.shallow-health-check", "warn", file, lineNo, "health/readiness path returns static OK while dependency evidence exists in the file", "medium", "healthcheck", "static-ok"))
			healthRegionLine = 0
		}
		if enabled(rules.DetectCriticalPathUninstrumented) && isCriticalPath(file, codeLine, rules) && !hasInstrumentation {
			findings = append(findings, newFinding(env, "observability.critical-path-uninstrumented", "warn", file, lineNo, "critical production path lacks visible metrics, tracing, or structured logging evidence", "medium", "critical_path", criticalKind(file, codeLine, rules)))
		}
	}

	return support.DedupeFindings(findings, func(finding core.Finding) string {
		return finding.RuleID + "|" + finding.Path + "|" + fmt.Sprintf("%d", finding.Line)
	})
}

func isObservableSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".py", ".ts", ".tsx", ".js", ".jsx", ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx", ".h":
		return true
	default:
		return false
	}
}

func maskStringsForStructure(file string, source string) string {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".ts", ".tsx", ".js", ".jsx":
		return support.StripTypeScriptCommentsAndStrings(source)
	default:
		return source
	}
}

func isRawLogLine(line string) bool {
	for _, pattern := range rawLogPatterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

func isLogLikeLine(line string) bool {
	lower := strings.ToLower(line)
	return isRawLogLine(line) || strings.Contains(lower, "logger.") || strings.Contains(lower, "logging.") || strings.Contains(lower, "slog.") || strings.Contains(lower, "zap.")
}

func hasStructuredContext(line string) bool {
	return strings.Contains(line, "{") || strings.Contains(line, "With(") || strings.Contains(line, "WithFields") || strings.Contains(line, "String(") || strings.Contains(line, "Int(") || strings.Contains(line, "extra=")
}

func hasErrorContext(line string, rules core.ObservabilityRulesConfig) bool {
	lower := strings.ToLower(line)
	if observabilityContainsAny(lower, "operation", "op", "request", "request_id", "trace", "span", "route", "handler", "job", "consumer", "customer", "account", "order") {
		return true
	}
	for _, pattern := range rules.InstrumentationEvidencePatterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern != "" && pattern != "logger" && pattern != "logging" && strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func firstPatternEvidence(line string, patterns []string) (string, bool) {
	lower := strings.ToLower(line)
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern != "" && strings.Contains(lower, pattern) {
			return pattern, true
		}
	}
	return "", false
}

func nextFewLinesIgnoreError(lines []string, idx int) bool {
	for next := idx + 1; next < len(lines) && next <= idx+3; next++ {
		if logAndIgnoreNextPattern.MatchString(lines[next]) {
			return true
		}
	}
	return false
}

func isHealthPathOrLine(file string, line string, rules core.ObservabilityRulesConfig) bool {
	lower := strings.ToLower(file + "\n" + line)
	for _, pattern := range rules.HealthcheckPathPatterns {
		if strings.TrimSpace(pattern) != "" && strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func isCriticalPath(file string, line string, rules core.ObservabilityRulesConfig) bool {
	lower := strings.ToLower(file + "\n" + line)
	if !observabilityContainsAny(lower, "func ", "function ", "def ", "=>", "::") {
		return false
	}
	for _, pattern := range rules.CriticalPathPatterns {
		if strings.TrimSpace(pattern) != "" && strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func criticalKind(file string, line string, rules core.ObservabilityRulesConfig) string {
	lower := strings.ToLower(file + "\n" + line)
	for _, pattern := range rules.CriticalPathPatterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern != "" && strings.Contains(lower, pattern) {
			return pattern
		}
	}
	return "critical-path"
}

func hasInstrumentationEvidence(source string, rules core.ObservabilityRulesConfig) bool {
	lower := strings.ToLower(source)
	for _, pattern := range rules.InstrumentationEvidencePatterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern != "" && strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func isCommentLine(line string) bool {
	return strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "*")
}

func isTestPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "testdata/") || strings.Contains(lower, "__tests__/") || strings.Contains(lower, "fixtures/") || strings.HasSuffix(lower, "_test.go") || strings.HasSuffix(lower, "_test.py") || strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.")
}

func observabilityContainsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func enabled(toggle *bool) bool {
	return toggle == nil || *toggle
}

func newFinding(env support.Context, ruleID string, level string, path string, line int, message string, confidence string, metaKey string, metaValue string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:     ruleID,
		Level:      level,
		Path:       path,
		Line:       line,
		Column:     1,
		Message:    message,
		Confidence: confidence,
		Metadata:   map[string]string{metaKey: metaValue},
	})
}
