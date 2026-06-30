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

func typeScriptAIOnlyFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	return typeScriptAIQualityFindings(typeScriptScanContext{
		env:    env,
		file:   file,
		source: source,
		code:   support.StripTypeScriptCommentsAndStrings(source),
	})
}

func qualityRuleID(path string, suffix string) string {
	return support.RuleIDForScript(path, "quality.typescript."+suffix, "quality.javascript."+suffix)
}

func newTypeScriptQualityFinding(ctx typeScriptScanContext, ruleID string, line int, message string) core.Finding {
	return warnFinding(ctx.env, ruleID, ctx.file, line, 1, support.ScriptLabelForPath(ctx.file)+" "+message)
}

func isTypeScriptLikeFile(rel string) bool {
	return support.IsTypeScriptLikeFile(rel)
}
