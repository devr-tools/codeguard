package quality

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	tsExplicitAnyPattern    = regexp.MustCompile(`(?:[:<,(]\s*any\b|\bas\s+any\b)`)
	tsDoubleAssertPattern   = regexp.MustCompile(`\bas\s+(?:unknown|any)\s+as\s+`)
	tsIgnoreCommentPattern  = regexp.MustCompile(`^\s*(?://|/\*+|\*)\s*@ts-ignore\b`)
	tsNoCheckCommentPattern = regexp.MustCompile(`^\s*(?://|/\*+|\*)\s*@ts-nocheck\b`)
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
			findings = append(findings, ctx.env.NewFinding(support.FindingInput{
				RuleID:  "quality.typescript.ts-ignore",
				Level:   "warn",
				Path:    ctx.file,
				Line:    idx + 1,
				Column:  1,
				Message: "TypeScript suppression comment should be reviewed",
			}))
		case tsNoCheckCommentPattern.MatchString(line):
			findings = append(findings, ctx.env.NewFinding(support.FindingInput{
				RuleID:  "quality.typescript.ts-nocheck",
				Level:   "warn",
				Path:    ctx.file,
				Line:    idx + 1,
				Column:  1,
				Message: "TypeScript file-level type checking is disabled",
			}))
		}
	}
	return findings
}

func typeScriptPatternFindings(ctx typeScriptScanContext) []core.Finding {
	findings := make([]core.Finding, 0, 3)
	findings = append(findings, regexTypeScriptFinding(ctx, typeScriptPatternFinding{
		pattern: tsExplicitAnyPattern,
		ruleID:  "quality.typescript.explicit-any",
		level:   "warn",
		message: "explicit any should be reviewed",
	})...)
	findings = append(findings, regexTypeScriptFinding(ctx, typeScriptPatternFinding{
		pattern: tsDoubleAssertPattern,
		ruleID:  "quality.typescript.double-assertion",
		level:   "warn",
		message: "double type assertions should be reviewed",
	})...)
	for _, line := range typeScriptNonNullAssertionLines(ctx.code) {
		findings = append(findings, ctx.env.NewFinding(support.FindingInput{
			RuleID:  "quality.typescript.non-null-assertion",
			Level:   "warn",
			Path:    ctx.file,
			Line:    line,
			Column:  1,
			Message: "non-null assertions should be reviewed",
		}))
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
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts":
		return true
	default:
		return false
	}
}
