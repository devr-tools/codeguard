package quality

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	tsExplicitAnyPattern     = regexp.MustCompile(`(?:[:<,(]\s*any\b|\bas\s+any\b)`)
	tsDoubleAssertPattern    = regexp.MustCompile(`\bas\s+(?:unknown|any)\s+as\s+`)
	tsDebuggerPattern        = regexp.MustCompile(`\bdebugger\s*;?`)
	tsIgnoreCommentPattern   = regexp.MustCompile(`^\s*(?://|/\*+|\*)\s*@ts-ignore\b`)
	tsNoCheckCommentPattern  = regexp.MustCompile(`^\s*(?://|/\*+|\*)\s*@ts-nocheck\b`)
	tsExpectErrorCommentRule = regexp.MustCompile(`^\s*(?://|/\*+|\*)\s*@ts-expect-error\b`)
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

func regexTypeScriptFinding(ctx typeScriptScanContext, spec typeScriptPatternFinding) []core.Finding {
	matches := spec.pattern.FindAllStringIndex(ctx.code, -1)
	if len(matches) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0, len(matches))
	seenLines := make(map[int]struct{}, len(matches))
	for _, match := range matches {
		line := support.LineNumberForOffset(ctx.source, match[0])
		if _, exists := seenLines[line]; exists {
			continue
		}
		seenLines[line] = struct{}{}
		findings = append(findings, ctx.env.NewFinding(support.FindingInput{
			RuleID:  spec.ruleID,
			Level:   spec.level,
			Path:    ctx.file,
			Line:    line,
			Column:  1,
			Message: spec.message,
		}))
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

func isTypeScriptLikeFile(rel string) bool {
	return support.IsTypeScriptLikeFile(rel)
}

func qualityRuleID(path string, suffix string) string {
	return support.RuleIDForScript(path, "quality.typescript."+suffix, "quality.javascript."+suffix)
}

func newTypeScriptQualityFinding(ctx typeScriptScanContext, ruleID string, line int, message string) core.Finding {
	return ctx.env.NewFinding(support.FindingInput{
		RuleID:  ruleID,
		Level:   "warn",
		Path:    ctx.file,
		Line:    line,
		Column:  1,
		Message: support.ScriptLabelForPath(ctx.file) + " " + message,
	})
}
