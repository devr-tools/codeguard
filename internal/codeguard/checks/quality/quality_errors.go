package quality

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const (
	errorLoggedAndReturnedRuleID         = "error.logged-and-returned"
	errorGenericMessageRuleID            = "error.generic-message"
	errorWrongAbstractionLevelRuleID     = "error.wrong-abstraction-level"
	errorInconsistentWrappingRuleID      = "error.inconsistent-wrapping"
	errorRetryableNotDistinguishedRuleID = "error.retryable-not-distinguished"
	errorUserMessageLeaksInternalsRuleID = "error.user-message-leaks-internals"
	errorPartialFailureHiddenRuleID      = "error.partial-failure-hidden"
	errorCleanupErrorIgnoredRuleID       = "error.cleanup-error-ignored"
	errorPanicOnRecoverablePathRuleID    = "error.panic-on-recoverable-path"
	errorExceptionControlFlowRuleID      = "error.exception-used-for-control-flow"
	errorFallbackHidesCorruptionRuleID   = "error.fallback-hides-corruption"
)

var (
	genericErrorConstructorPattern = regexp.MustCompile(`(?i)(errors\.New|fmt\.Errorf|new\s+Error|Exception|runtime_error|std::runtime_error|throw)\s*\(\s*["']([^"']+)["']`)
	errorStringPattern             = regexp.MustCompile(`["']([^"']{3,160})["']`)
	internalErrorLeakPattern       = regexp.MustCompile(`(?i)\b(sql|sqlite|postgres|postgresql|mysql|redis|kafka|grpc|stack trace|traceback|errno|database|db\.|pq:|driver:|dial tcp|connection refused)\b`)
	wrappedErrorPattern            = regexp.MustCompile(`(?i)(%w|errors\.Wrap|fmt\.Errorf\s*\([^)]*err|raise\s+\w+.*\s+from\s+\w+|cause\s*:)`)
	cleanupIgnoredPattern          = regexp.MustCompile(`(?i)(_\s*=\s*[^;\n]*(close|rollback|remove|delete)\s*\(|defer\s+[^;\n]*\.close\s*\(|catch\s*\([^)]*\)\s*\{\s*(?:/\*.*\*/|//.*)?\s*\})`)
	panicPattern                   = regexp.MustCompile(`\bpanic\s*\(`)
	throwRaisePattern              = regexp.MustCompile(`(?i)\b(throw|raise)\b`)
)

func errorContractFindings(env support.Context, file string, fn precisionFunction) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	body := functionRawBody(fn)
	loweredBody := strings.ToLower(body)
	findings := make([]core.Finding, 0)

	if line, ok := loggedAndReturnedLine(fn.Statements); ok {
		findings = append(findings, precisionWarnFinding(env, errorLoggedAndReturnedRuleID, file, line,
			"error is logged and returned from the same boundary, risking duplicate logs", core.ConfidenceHigh))
	}
	if line, ok := genericErrorMessageLine(fn.Statements); ok {
		findings = append(findings, precisionWarnFinding(env, errorGenericMessageRuleID, file, line,
			"error message is generic and lacks operation, resource, or decision context", core.ConfidenceMedium))
	}
	if line, ok := inconsistentWrappingLine(fn); ok {
		findings = append(findings, precisionWarnFinding(env, errorInconsistentWrappingRuleID, file, line,
			"function mixes wrapped errors with bare error returns, making the error contract inconsistent", core.ConfidenceMedium))
	}
	if line, ok := cleanupIgnoredLine(fn.Statements); ok {
		findings = append(findings, precisionWarnFinding(env, errorCleanupErrorIgnoredRuleID, file, line,
			"cleanup error is discarded; close, rollback, or delete failures should be handled or joined", core.ConfidenceHigh))
	}
	if line, ok := partialFailureHiddenLine(fn, loweredBody); ok {
		findings = append(findings, precisionWarnFinding(env, errorPartialFailureHiddenRuleID, file, line,
			"partial failure path continues or returns success without surfacing the failed work", core.ConfidenceMedium))
	}
	if line, ok := fallbackHidesCorruptionLine(fn, loweredBody); ok {
		findings = append(findings, precisionWarnFinding(env, errorFallbackHidesCorruptionRuleID, file, line,
			"fallback success after parse, corruption, or validation failure can hide bad data", core.ConfidenceMedium))
	}
	if line, ok := retryableUndistinguishedLine(fn, loweredBody); ok {
		findings = append(findings, precisionWarnFinding(env, errorRetryableNotDistinguishedRuleID, file, line,
			"retry path does not distinguish transient from permanent failures", core.ConfidenceMedium))
	}
	if line, ok := wrongAbstractionLevelLine(fn, body); ok {
		findings = append(findings, precisionWarnFinding(env, errorWrongAbstractionLevelRuleID, file, line,
			"error contract exposes lower-level infrastructure details at a higher abstraction boundary", core.ConfidenceMedium))
	}
	if line, ok := userMessageLeakLine(fn, body); ok {
		findings = append(findings, precisionWarnFinding(env, errorUserMessageLeaksInternalsRuleID, file, line,
			"user-facing error message leaks database, transport, or stack internals", core.ConfidenceHigh))
	}
	if line, ok := panicOnRecoverableLine(fn, body); ok {
		findings = append(findings, precisionWarnFinding(env, errorPanicOnRecoverablePathRuleID, file, line,
			"panic is used on a recoverable request, validation, or I/O path", core.ConfidenceMedium))
	}
	if line, ok := exceptionControlFlowLine(fn); ok {
		findings = append(findings, precisionWarnFinding(env, errorExceptionControlFlowRuleID, file, line,
			"exception is used for ordinary branch control instead of explicit result handling", core.ConfidenceMedium))
	}
	return findings
}

func functionRawBody(fn precisionFunction) string {
	lines := make([]string, 0, len(fn.Statements))
	for _, statement := range fn.Statements {
		lines = append(lines, firstNonEmptyString(statement.Raw, statement.Text))
	}
	if len(lines) == 0 {
		return fn.Body
	}
	return strings.Join(lines, "\n")
}

func loggedAndReturnedLine(statements []support.ParsedStatement) (int, bool) {
	for idx, statement := range statements {
		if !logsError(firstNonEmptyString(statement.Raw, statement.Text)) {
			continue
		}
		if nearbyReturnedError(statements, idx) {
			return statement.Line, true
		}
	}
	return 0, false
}

func nearbyReturnedError(statements []support.ParsedStatement, idx int) bool {
	for lookahead := idx; lookahead < len(statements) && lookahead <= idx+4; lookahead++ {
		line := strings.TrimSpace(firstNonEmptyString(statements[lookahead].Raw, statements[lookahead].Text))
		if returnsBareError(line) || throwsBareError(line) || bareErrorReturn(line) {
			return true
		}
		lowered := strings.ToLower(line)
		if strings.HasPrefix(lowered, "return ") && strings.Contains(lowered, "err") &&
			!strings.Contains(lowered, "return nil") && !strings.Contains(lowered, "return none") {
			return true
		}
	}
	return false
}

func genericErrorMessageLine(statements []support.ParsedStatement) (int, bool) {
	for _, statement := range statements {
		raw := firstNonEmptyString(statement.Raw, statement.Text)
		match := genericErrorConstructorPattern.FindStringSubmatch(raw)
		if match == nil {
			continue
		}
		if genericErrorMessage(match[2]) {
			return statement.Line, true
		}
	}
	return 0, false
}

func genericErrorMessage(message string) bool {
	normalized := strings.TrimSpace(strings.ToLower(message))
	if normalized == "" {
		return false
	}
	for _, generic := range []string{
		"error", "failed", "failure", "invalid", "bad request", "request failed",
		"operation failed", "something went wrong", "unknown error", "not found",
		"unauthorized", "forbidden", "oops",
	} {
		if normalized == generic {
			return true
		}
	}
	return len(strings.Fields(normalized)) <= 2 &&
		containsAny(normalized, []string{"failed", "invalid", "error", "failure"})
}

func inconsistentWrappingLine(fn precisionFunction) (int, bool) {
	if !wrappedErrorPattern.MatchString(functionRawBody(fn)) {
		return 0, false
	}
	for _, statement := range fn.Statements {
		line := firstNonEmptyString(statement.Raw, statement.Text)
		if bareErrorReturn(line) || returnsBareError(line) || throwsBareError(line) {
			return statement.Line, true
		}
	}
	return 0, false
}

func bareErrorReturn(line string) bool {
	trimmed := strings.TrimSpace(strings.TrimSuffix(line, ";"))
	lowered := strings.ToLower(trimmed)
	return regexp.MustCompile(`^return\s+(.+,\s*)?(err|error|e)$`).MatchString(lowered) ||
		lowered == "raise" || lowered == "raise e" || lowered == "throw e"
}

func cleanupIgnoredLine(statements []support.ParsedStatement) (int, bool) {
	for _, statement := range statements {
		if cleanupIgnoredPattern.MatchString(firstNonEmptyString(statement.Raw, statement.Text)) {
			return statement.Line, true
		}
	}
	return 0, false
}

func partialFailureHiddenLine(fn precisionFunction, loweredBody string) (int, bool) {
	if strings.Contains(loweredBody, "allsettled") && !containsAny(loweredBody, []string{"rejected", "throw", "return err", "return error"}) {
		return fn.StartLine, true
	}
	if !containsAny(loweredBody, []string{"continue", "pass"}) || !containsAny(loweredBody, []string{"err", "error", "catch", "except"}) {
		return 0, false
	}
	if containsAny(loweredBody, []string{"return err", "return error", "throw", "raise"}) {
		return 0, false
	}
	for _, statement := range fn.Statements {
		lowered := strings.ToLower(firstNonEmptyString(statement.Raw, statement.Text))
		if strings.Contains(lowered, "continue") || strings.TrimSpace(lowered) == "pass" {
			return statement.Line, true
		}
	}
	return 0, false
}

func fallbackHidesCorruptionLine(fn precisionFunction, loweredBody string) (int, bool) {
	if !containsAny(loweredBody, []string{"json", "parse", "deserialize", "unmarshal", "decode", "validate", "corrupt"}) {
		return 0, false
	}
	if !containsAny(loweredBody, []string{"catch", "except", "err != nil", "error"}) {
		return 0, false
	}
	for _, statement := range fn.Statements {
		lowered := strings.ToLower(firstNonEmptyString(statement.Raw, statement.Text))
		if containsAny(lowered, []string{"return {}", "return []", "return map[", "return default", "return fallback"}) {
			return statement.Line, true
		}
	}
	return 0, false
}

func retryableUndistinguishedLine(fn precisionFunction, loweredBody string) (int, bool) {
	if !containsAny(loweredBody, []string{"retry", "backoff", "again", "attempt"}) ||
		!containsAny(loweredBody, []string{"err", "error", "catch", "except", "failure"}) {
		return 0, false
	}
	if containsAny(loweredBody, []string{"retryable", "transient", "permanent", "temporary", "timeout", "status", "rate limit"}) {
		return 0, false
	}
	return fn.StartLine, true
}

func wrongAbstractionLevelLine(fn precisionFunction, body string) (int, bool) {
	if !internalErrorLeakPattern.MatchString(body) {
		return 0, false
	}
	if boundaryFunctionName(fn.Name) || domainFunctionName(fn.Name) {
		return firstErrorStringLine(fn), true
	}
	return 0, false
}

func userMessageLeakLine(fn precisionFunction, body string) (int, bool) {
	if !internalErrorLeakPattern.MatchString(body) {
		return 0, false
	}
	if !boundaryFunctionName(fn.Name) && !containsAny(strings.ToLower(body), []string{"http.error", "response", "json", "status", "usermessage", "user_message"}) {
		return 0, false
	}
	return firstErrorStringLine(fn), true
}

func firstErrorStringLine(fn precisionFunction) int {
	for _, statement := range fn.Statements {
		raw := firstNonEmptyString(statement.Raw, statement.Text)
		if errorStringPattern.MatchString(raw) || internalErrorLeakPattern.MatchString(raw) {
			return statement.Line
		}
	}
	return fn.StartLine
}

func panicOnRecoverableLine(fn precisionFunction, body string) (int, bool) {
	if !panicPattern.MatchString(body) || strings.HasPrefix(strings.ToLower(fn.Name), "must") {
		return 0, false
	}
	if !containsAny(strings.ToLower(fn.Name), []string{"handle", "parse", "load", "save", "validate", "decode", "request", "process"}) {
		return 0, false
	}
	for _, statement := range fn.Statements {
		if panicPattern.MatchString(firstNonEmptyString(statement.Raw, statement.Text)) {
			return statement.Line, true
		}
	}
	return fn.StartLine, true
}

func exceptionControlFlowLine(fn precisionFunction) (int, bool) {
	for idx, statement := range fn.Statements {
		raw := firstNonEmptyString(statement.Raw, statement.Text)
		lowered := strings.ToLower(raw)
		if !throwRaisePattern.MatchString(raw) || !containsAny(lowered, []string{"invalid", "not found", "missing", "stop", "continue", "break"}) {
			continue
		}
		if strings.Contains(lowered, "panic(") {
			continue
		}
		if strings.Contains(lowered, " if ") || strings.HasPrefix(strings.TrimSpace(lowered), "if ") ||
			(idx > 0 && strings.HasPrefix(strings.TrimSpace(strings.ToLower(fn.Statements[idx-1].Text)), "if ")) {
			return statement.Line, true
		}
	}
	return 0, false
}

func boundaryFunctionName(name string) bool {
	lowered := strings.ToLower(name)
	return containsAny(lowered, []string{"handler", "handle", "controller", "route", "api", "endpoint", "render", "respond", "response"})
}

func domainFunctionName(name string) bool {
	lowered := strings.ToLower(name)
	return containsAny(lowered, []string{"checkout", "order", "profile", "account", "customer", "payment", "invoice", "subscription"})
}
