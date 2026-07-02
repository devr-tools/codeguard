package security

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// typeScriptHTMLSinkQuery captures the HTML-injection sinks the regex path
// looks for, but with syntactic roles (docs/treesitter-spike.md §6.1): plain
// and compound assignments to element HTML properties (a comparison like
// `el.innerHTML === ""` cannot match), plus DOM API calls that write markup.
// The object/property split lets the classifier require `document` as the
// receiver for write/writeln even when a formatter splits the member chain
// across lines. The same query compiles against the typescript, tsx, and
// javascript grammars, so the tree path also serves the
// security.javascript.unsafe-html-sink mirror.
var typeScriptHTMLSinkQuery = support.CompileScriptQuery(`
(assignment_expression
  left: (member_expression
    property: (property_identifier) @sink.assign))
(augmented_assignment_expression
  left: (member_expression
    property: (property_identifier) @sink.assign))
(call_expression
  function: (member_expression
    object: (_) @sink.object
    property: (property_identifier) @sink.method))
`)

func typeScriptUnsafeHTMLSinkFindings(ctx typeScriptScanContext, tree *support.SyntaxTree) []core.Finding {
	regexSpec := support.ScriptRegexSpec{
		Pattern: typeScriptUnsafeHTMLPattern,
		RuleID:  securityRuleID(ctx.file, "unsafe-html-sink"),
		Level:   "warn",
		Message: "unsafe HTML injection sink should be reviewed",
	}
	if tree != nil {
		findings, ok := support.ScriptQueryFindings(ctx.env, ctx.file, tree, support.ScriptQuerySpec{
			Query:      typeScriptHTMLSinkQuery,
			RuleID:     regexSpec.RuleID,
			Level:      regexSpec.Level,
			Message:    regexSpec.Message,
			Confidence: core.ConfidenceHigh,
			Classify:   classifyHTMLSinkHit,
		})
		if ok {
			return findings
		}
	}
	return regexTypeScriptSecurityFindings(ctx, regexSpec)
}

// classifyHTMLSinkHit keeps assignments to innerHTML/outerHTML, any
// insertAdjacentHTML call, and write/writeln calls whose receiver is the
// document object (mirroring the `\bdocument\.` intent of the regex, which
// also fires on window.document.write).
func classifyHTMLSinkHit(hit support.QueryHit) (int, bool) {
	object := hit.CaptureText("sink.object")
	for _, capture := range hit.Captures {
		switch capture.Name {
		case "sink.assign":
			if capture.Text == "innerHTML" || capture.Text == "outerHTML" {
				return capture.Line, true
			}
		case "sink.method":
			if capture.Text == "insertAdjacentHTML" {
				return capture.Line, true
			}
			if (capture.Text == "write" || capture.Text == "writeln") && isDocumentReceiver(object) {
				return capture.Line, true
			}
		}
	}
	return 0, false
}

func isDocumentReceiver(text string) bool {
	return text == "document" || strings.HasSuffix(text, ".document")
}
