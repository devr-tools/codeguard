package quality

import (
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

func typeScriptFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0) //nolint:prealloc // count not known up front; each scan stage appends a variable number
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
	return append(fileLengthFindingWithSignals(env, file, data, findings), findings...)
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
	// Tree-sitter path for the migrated rules (nil tree means regex fallback;
	// see support.ScriptSyntaxTree). The tree is parsed once per file per scan
	// by the runner's corpus cache.
	tree := support.ScriptSyntaxTree(ctx.env, ctx.file, ctx.source)
	findings := make([]core.Finding, 0, 4)
	findings = append(findings, typeScriptExplicitAnyFindings(ctx, tree)...)
	findings = append(findings, typeScriptDoubleAssertionFindings(ctx, tree)...)
	findings = append(findings, regexTypeScriptFinding(ctx, support.ScriptRegexSpec{
		Pattern: tsDebuggerPattern,
		RuleID:  qualityRuleID(ctx.file, "debugger-statement"),
		Level:   "warn",
		Message: "debugger statements should not reach committed source",
	})...)
	findings = append(findings, typeScriptNonNullAssertionFindings(ctx, tree)...)
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

func regexTypeScriptFinding(ctx typeScriptScanContext, spec support.ScriptRegexSpec) []core.Finding {
	return support.ScriptRegexFindings(ctx.env, ctx.file, support.ScriptScanContext{Source: ctx.source, Code: ctx.code}, spec)
}
