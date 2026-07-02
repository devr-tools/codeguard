package quality

import (
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Tree-sitter queries for the migrated TypeScript quality rules
// (docs/treesitter-spike.md §6.1). Each rule keeps its regex implementation
// as the fallback for parsers.treesitter "off", non-TS files, and any
// per-file parse refusal; when the tree path runs, findings keep the same
// rule ID, level, and message and gain Confidence "high" because the grammar
// removes the regex false-positive classes (identifiers named `any`, regex
// literal bodies, `!=`/`!!` operators) by construction.
var (
	// tsExplicitAnyQuery captures every predefined-type use; the classifier
	// keeps the ones whose text is exactly `any`. Matching the grammar's type
	// position (annotations, generics, as/satisfies expressions) is what
	// distinguishes `x: any` from a value identifier that happens to be
	// named any.
	tsExplicitAnyQuery = support.CompileScriptQuery(`(predefined_type) @any.type`)

	// tsDoubleAssertQuery captures nested as-expressions and the inner cast's
	// type; the classifier keeps `as unknown as`/`as any as` chains.
	tsDoubleAssertQuery = support.CompileScriptQuery(`
(as_expression
  (as_expression
    [(predefined_type) (type_identifier)] @dbl.type))
`)

	// tsNonNullQuery captures postfix non-null assertions plus definite
	// assignment assertions (`let ready!: boolean`, `name!: string` class
	// fields), which the regex path also reports. `!=` and `!!` cannot match:
	// the grammar only produces these nodes for genuine assertions (a nested
	// `x!!` yields two nodes on the same line, which the per-line dedupe
	// collapses like the regex path).
	tsNonNullQuery = support.CompileScriptQuery(`
(non_null_expression) @nna
(variable_declarator "!" @nna.def)
(public_field_definition "!" @nna.def)
`)
)

// typeScriptTreeUsable reports whether the tree path applies to a TS-only
// rule: the grammars that model TypeScript syntax are typescript and tsx.
// JavaScript files keep their regex path (`any`, `as`, and `!` assertions
// are TypeScript syntax, so the TS-only rules cannot be expressed against
// the javascript grammar).
func typeScriptTreeUsable(tree *support.SyntaxTree) bool {
	if tree == nil {
		return false
	}
	lang := tree.Language()
	return lang == support.ScriptLangTypeScript || lang == support.ScriptLangTSX
}

func typeScriptExplicitAnyFindings(ctx typeScriptScanContext, tree *support.SyntaxTree) []core.Finding {
	regexSpec := support.ScriptRegexSpec{
		Pattern: tsExplicitAnyPattern,
		RuleID:  qualityRuleID(ctx.file, "explicit-any"),
		Level:   "warn",
		Message: "explicit any should be reviewed",
	}
	if typeScriptTreeUsable(tree) {
		findings, ok := support.ScriptQueryFindings(ctx.env, ctx.file, tree, support.ScriptQuerySpec{
			Query:      tsExplicitAnyQuery,
			RuleID:     regexSpec.RuleID,
			Level:      regexSpec.Level,
			Message:    regexSpec.Message,
			Confidence: core.ConfidenceHigh,
			Classify: func(hit support.QueryHit) (int, bool) {
				for _, capture := range hit.Captures {
					if capture.Name == "any.type" && capture.Text == "any" {
						return capture.Line, true
					}
				}
				return 0, false
			},
		})
		if ok {
			return findings
		}
	}
	return regexTypeScriptFinding(ctx, regexSpec)
}

func typeScriptDoubleAssertionFindings(ctx typeScriptScanContext, tree *support.SyntaxTree) []core.Finding {
	regexSpec := support.ScriptRegexSpec{
		Pattern: tsDoubleAssertPattern,
		RuleID:  qualityRuleID(ctx.file, "double-assertion"),
		Level:   "warn",
		Message: "double type assertions should be reviewed",
	}
	if typeScriptTreeUsable(tree) {
		findings, ok := support.ScriptQueryFindings(ctx.env, ctx.file, tree, support.ScriptQuerySpec{
			Query:      tsDoubleAssertQuery,
			RuleID:     regexSpec.RuleID,
			Level:      regexSpec.Level,
			Message:    regexSpec.Message,
			Confidence: core.ConfidenceHigh,
			Classify: func(hit support.QueryHit) (int, bool) {
				for _, capture := range hit.Captures {
					if capture.Name == "dbl.type" && (capture.Text == "unknown" || capture.Text == "any") {
						return capture.Line, true
					}
				}
				return 0, false
			},
		})
		if ok {
			return findings
		}
	}
	return regexTypeScriptFinding(ctx, regexSpec)
}

func typeScriptNonNullAssertionFindings(ctx typeScriptScanContext, tree *support.SyntaxTree) []core.Finding {
	ruleID := qualityRuleID(ctx.file, "non-null-assertion")
	if typeScriptTreeUsable(tree) {
		findings, ok := support.ScriptQueryFindings(ctx.env, ctx.file, tree, support.ScriptQuerySpec{
			Query:      tsNonNullQuery,
			RuleID:     ruleID,
			Level:      "warn",
			Message:    support.ScriptLabelForPath(ctx.file) + " non-null assertions should be reviewed",
			Confidence: core.ConfidenceHigh,
			Classify: func(hit support.QueryHit) (int, bool) {
				for _, capture := range hit.Captures {
					switch capture.Name {
					case "nna":
						// The `!` token ends the expression, so report its
						// line (a formatter may split the operand across
						// lines; the assertion itself is at the end).
						return capture.EndLine, true
					case "nna.def":
						return capture.Line, true
					}
				}
				return 0, false
			},
		})
		if ok {
			return findings
		}
	}
	findings := make([]core.Finding, 0)
	for _, line := range typeScriptNonNullAssertionLines(ctx.code) {
		findings = append(findings, newTypeScriptQualityFinding(ctx, ruleID, line, "non-null assertions should be reviewed"))
	}
	return findings
}
