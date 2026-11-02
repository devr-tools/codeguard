package quality

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func errorHandlingFindings(env support.Context, file string, fn precisionFunction) []core.Finding {
	if isUIHelperOrMappingContext(file, fn) || isSeedOrScriptSourcePath(file) {
		return nil
	}
	findings := make([]core.Finding, 0)
	statements := fn.Statements
	for idx, statement := range statements {
		line := strings.TrimSpace(statement.Text)
		lowered := strings.ToLower(line)
		if strings.Contains(lowered, "err") || strings.Contains(lowered, "except") || strings.Contains(lowered, "catch") {
			if logsError(line) && nearbyIgnoredError(statements, idx) && !nearbyExplicitDegradedFallback(statements, idx) {
				findings = append(findings, precisionWarnFinding(env, errorLoggedAndIgnoredRuleID, file, statement.Line,
					"error is logged and then ignored or converted to success", core.ConfidenceHigh))
			}
			if returnsBareError(line) || throwsBareError(line) {
				findings = append(findings, precisionWarnFinding(env, errorContextLostRuleID, file, statement.Line,
					"error is returned without contextual wrapping", core.ConfidenceMedium))
			}
		}
	}
	return findings
}

func nearbyExplicitDegradedFallback(statements []support.ParsedStatement, idx int) bool {
	for lookahead := idx; lookahead < len(statements) && lookahead <= idx+4; lookahead++ {
		line := strings.ToLower(strings.TrimSpace(firstNonEmptyString(statements[lookahead].Raw, statements[lookahead].Text)))
		if containsAny(line, []string{
			"return []", "return {}", "return reports", "return fallback", "return default",
			"return;",
		}) {
			return true
		}
	}
	return false
}

func logsError(line string) bool {
	lowered := strings.ToLower(line)
	return strings.Contains(lowered, "log.") || strings.Contains(lowered, "logger.") ||
		strings.Contains(lowered, "logging.") || strings.Contains(lowered, "console.error")
}

func nearbyIgnoredError(statements []support.ParsedStatement, idx int) bool {
	for lookahead := idx; lookahead < len(statements) && lookahead <= idx+4; lookahead++ {
		line := strings.TrimSpace(statements[lookahead].Text)
		lowered := strings.ToLower(line)
		if strings.Contains(lowered, "return nil") || strings.Contains(lowered, "return none") ||
			strings.Contains(lowered, "return undefined") || lowered == "return;" ||
			lowered == "pass" || strings.Contains(lowered, "// ignore") {
			return true
		}
		if strings.HasPrefix(lowered, "return ") && !strings.Contains(lowered, "err") && !strings.Contains(lowered, "error") {
			return true
		}
	}
	return false
}

func returnsBareError(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "return err" || trimmed == "return err;" || trimmed == "return error" || trimmed == "return error;"
}

func throwsBareError(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "throw err;" || trimmed == "throw err" || trimmed == "throw error;" || trimmed == "throw error" ||
		trimmed == "raise err" || trimmed == "raise error"
}
