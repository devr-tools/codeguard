package quality

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type typeScriptScanContext struct {
	env    support.Context
	file   string
	source string
	code   string
}

type typeScriptPatternFinding struct {
	pattern *regexp.Regexp
	ruleID  string
	level   string
	message string
}

func typeScriptFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := fileLengthFinding(env, file, data)
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	ctx := typeScriptScanContext{
		env:    env,
		file:   file,
		source: source,
		code:   support.StripTypeScriptCommentsAndStrings(source),
	}

	findings = append(findings, appendTypeScriptDirectiveFindings(ctx)...)
	findings = append(findings, typeScriptPatternFindings(ctx)...)
	findings = append(findings, typeScriptAIQualityFindings(ctx)...)
	for _, fn := range typeScriptFunctions(source) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return findings
}

func appendTypeScriptDirectiveFindings(ctx typeScriptScanContext) []core.Finding {
	findings := make([]core.Finding, 0)
	for idx, line := range strings.Split(ctx.source, "\n") {
		switch {
		case tsIgnoreCommentPattern.MatchString(line):
			findings = append(findings, newTypeScriptQualityFinding(ctx, qualityRuleID(ctx.file, "ts-ignore"), idx+1, "suppression comment should be reviewed"))
		case tsNoCheckCommentPattern.MatchString(line):
			findings = append(findings, newTypeScriptQualityFinding(ctx, qualityRuleID(ctx.file, "ts-nocheck"), idx+1, "file-level type checking is disabled"))
		case tsExpectErrorCommentRule.MatchString(line):
			findings = append(findings, newTypeScriptQualityFinding(ctx, qualityRuleID(ctx.file, "ts-expect-error"), idx+1, "suppression comment should be reviewed"))
		}
	}
	return findings
}

func typeScriptPatternFindings(ctx typeScriptScanContext) []core.Finding {
	findings := make([]core.Finding, 0, 4)
	findings = append(findings, regexTypeScriptFinding(ctx, typeScriptPatternFinding{
		pattern: tsExplicitAnyPattern,
		ruleID:  qualityRuleID(ctx.file, "explicit-any"),
		level:   "warn",
		message: "explicit any should be reviewed",
	})...)
	findings = append(findings, regexTypeScriptFinding(ctx, typeScriptPatternFinding{
		pattern: tsDoubleAssertPattern,
		ruleID:  qualityRuleID(ctx.file, "double-assertion"),
		level:   "warn",
		message: "double type assertions should be reviewed",
	})...)
	findings = append(findings, regexTypeScriptFinding(ctx, typeScriptPatternFinding{
		pattern: tsDebuggerPattern,
		ruleID:  qualityRuleID(ctx.file, "debugger-statement"),
		level:   "warn",
		message: "debugger statements should not reach committed source",
	})...)
	for _, line := range typeScriptNonNullAssertionLines(ctx.code) {
		findings = append(findings, newTypeScriptQualityFinding(ctx, qualityRuleID(ctx.file, "non-null-assertion"), line, "non-null assertions should be reviewed"))
	}
	return findings
}

func typeScriptNonNullAssertionLines(code string) []int {
	lines := make([]int, 0)
	seen := make(map[int]struct{})
	for idx := 0; idx < len(code); idx++ {
		if code[idx] != '!' {
			continue
		}
		prev := support.PreviousSignificantByte(code, idx)
		next := support.NextSignificantByte(code, idx+1)
		if !support.IsTypeScriptAssertionTarget(prev) || next == '=' || next == '!' {
			continue
		}
		line := support.LineNumberForOffset(code, idx)
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	return lines
}

func regexTypeScriptFinding(ctx typeScriptScanContext, spec typeScriptPatternFinding) []core.Finding {
	return support.ScriptRegexFindings(ctx.env, ctx.file, support.ScriptScanContext{Source: ctx.source, Code: ctx.code}, support.ScriptRegexSpec{
		Pattern: spec.pattern,
		RuleID:  spec.ruleID,
		Level:   spec.level,
		Message: spec.message,
	})
}
