package quality

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func parsedDefensiveFindings(env support.Context, file string, parsed *support.ParsedFile) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, statement := range parsed.Module.Statements {
		findings = append(findings, defensiveStatementFindings(env, file, statement)...)
	}
	for _, fn := range parsed.AllFunctions() {
		for _, statement := range fn.Statements {
			findings = append(findings, defensiveStatementFindings(env, file, statement)...)
		}
	}
	return findings
}

func defensiveStatementFindings(env support.Context, file string, statement support.ParsedStatement) []core.Finding {
	text := statement.Text
	findings := make([]core.Finding, 0, 2)
	if strings.Contains(text, " as unknown as ") || strings.Contains(text, " as any as ") || strings.Contains(text, "typing.cast(") {
		if !isScriptTestOrHelperFile(file) {
			findings = append(findings, precisionWarnFinding(env, defensiveUncheckedTypeAssertionRuleID, file, statement.Line,
				"type assertion bypasses runtime validation", core.ConfidenceHigh))
		}
	}
	if unsafeScriptNumericConversion(text) {
		findings = append(findings, precisionWarnFinding(env, defensiveUnsafeNumericConversionRuleID, file, statement.Line,
			"numeric conversion can truncate, wrap, or lose precision; validate bounds before converting", core.ConfidenceMedium))
	}
	return findings
}

func unsafeScriptNumericConversion(text string) bool {
	lowered := strings.ToLower(text)
	return strings.Contains(lowered, "static_cast<int8_t>") ||
		strings.Contains(lowered, "static_cast<int16_t>") ||
		strings.Contains(lowered, "static_cast<int32_t>") ||
		strings.Contains(lowered, "static_cast<uint8_t>") ||
		strings.Contains(lowered, "static_cast<uint16_t>") ||
		strings.Contains(lowered, "static_cast<uint32_t>") ||
		strings.Contains(lowered, "number(") && strings.Contains(lowered, "bigint")
}

func parsedMutableGlobalFindings(env support.Context, file string, parsed *support.ParsedFile) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	if strings.HasPrefix(normalized, "bin/") || strings.Contains(normalized, "/integrations/") || strings.Contains(normalized, "integrations/") {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, statement := range parsed.Module.Statements {
		text := strings.TrimSpace(statement.Text)
		if text == "" || strings.HasPrefix(text, "const ") || strings.HasPrefix(text, "final ") {
			continue
		}
		if isScriptLikeSourcePath(file) && !moduleStatementLooksTopLevel(statement) {
			continue
		}
		if mutableGlobalLinePattern.MatchString(text) {
			findings = append(findings, precisionWarnFinding(env, qualityMutableGlobalStateRuleID, file, statement.Line,
				"mutable module-level state makes behavior harder to isolate and test", core.ConfidenceHigh))
		}
	}
	return findings
}

func sourceMutableGlobalFindings(env support.Context, file string, source string) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	if strings.EqualFold(filepath.Ext(file), ".go") {
		return nil
	}
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	if strings.HasPrefix(normalized, "bin/") || strings.Contains(normalized, "/integrations/") || strings.Contains(normalized, "integrations/") {
		return nil
	}
	var moduleScope []bool
	if isScriptLikeSourcePath(file) {
		moduleScope = scriptSourceModuleScopeLines(source)
	}
	for idx, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		if moduleScope != nil && !moduleScope[idx] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "const ") || strings.HasPrefix(trimmed, "final ") {
			continue
		}
		if mutableGlobalLinePattern.MatchString(trimmed) && !strings.Contains(trimmed, " := ") {
			return []core.Finding{precisionWarnFinding(env, qualityMutableGlobalStateRuleID, file, idx+1,
				"mutable module-level state makes behavior harder to isolate and test", core.ConfidenceHigh)}
		}
	}
	return nil
}

func redundantCommentFindings(env support.Context, file string, source string) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	if isSeedOrScriptSourcePath(file) {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	for idx := 0; idx+1 < len(lines); idx++ {
		comment := strings.TrimSpace(lines[idx])
		next := strings.TrimSpace(lines[idx+1])
		if !redundantCommentPattern.MatchString(comment) || next == "" {
			continue
		}
		verb := redundantCommentVerb(comment)
		if verb != "" && strings.Contains(strings.ToLower(next), verb) {
			return []core.Finding{precisionWarnFinding(env, qualityRedundantCommentRuleID, file, idx+1,
				"comment restates the next line without adding design intent or constraints", core.ConfidenceLow)}
		}
	}
	return nil
}

func redundantCommentVerb(comment string) string {
	match := redundantCommentPattern.FindStringSubmatch(comment)
	if len(match) < 3 {
		return ""
	}
	return strings.ToLower(match[2])
}
