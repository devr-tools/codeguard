// Package treesitter is a design-spike prototype evaluating tree-sitter as
// the parsing substrate for codeguard's non-Go language rules. It is a
// standalone Go module so its third-party dependencies never touch the root
// module's go.mod/go.sum; the root `go build ./...` and `go test ./...`
// skip this directory entirely. See docs/treesitter-spike.md for the full
// write-up and instructions for running the tests and benchmarks here.
package treesitter

import "sort"

// Finding is the minimal rule-hit shape the spike compares across engines:
// a rule identifier plus a 1-based line number, mirroring how the real
// checks dedupe regex hits per line.
type Finding struct {
	Rule string
	Line int
}

// Rule identifiers reimplemented by this spike. They correspond to
// quality.<file>.explicit-any and security.<file>.unsafe-html-sink in the
// real rule catalog.
const (
	RuleExplicitAny    = "explicit-any"
	RuleUnsafeHTMLSink = "unsafe-html-sink"
)

// explicitAnyQuery captures every use of a predefined type; the shared
// classifier keeps only the ones whose text is exactly `any`. Using the
// grammar's type context (rather than token text) is what distinguishes
// `x: any` from an identifier that happens to be named any.
const explicitAnyQuery = `(predefined_type) @any.type`

// htmlSinkQuery captures HTML-injection sinks: plain and compound
// assignments to element HTML properties, plus DOM API calls that write
// markup. Object/property split lets the classifier require `document`
// as the receiver for write/writeln.
const htmlSinkQuery = `
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
`

// capturedNode is the engine-independent view of one query capture that the
// shared classifier needs: capture name, node text, and 1-based line.
type capturedNode struct {
	capture string
	text    string
	line    int
}

// classifyExplicitAny converts an explicitAnyQuery capture into a finding.
func classifyExplicitAny(node capturedNode) (Finding, bool) {
	if node.capture == "any.type" && node.text == "any" {
		return Finding{Rule: RuleExplicitAny, Line: node.line}, true
	}
	return Finding{}, false
}

// classifyHTMLSink converts one htmlSinkQuery match (grouped captures) into
// a finding. objectText is the receiver text for method-call matches and
// empty for assignment matches.
func classifyHTMLSink(node capturedNode, objectText string) (Finding, bool) {
	switch node.capture {
	case "sink.assign":
		if node.text == "innerHTML" || node.text == "outerHTML" {
			return Finding{Rule: RuleUnsafeHTMLSink, Line: node.line}, true
		}
	case "sink.method":
		if node.text == "insertAdjacentHTML" {
			return Finding{Rule: RuleUnsafeHTMLSink, Line: node.line}, true
		}
		if (node.text == "write" || node.text == "writeln") && isDocumentReceiver(objectText) {
			return Finding{Rule: RuleUnsafeHTMLSink, Line: node.line}, true
		}
	}
	return Finding{}, false
}

// isDocumentReceiver reports whether the call receiver is the document
// object, matching the `\bdocument\.` intent of the current regex (which
// also fires on window.document.write).
func isDocumentReceiver(text string) bool {
	return text == "document" || hasSuffix(text, ".document")
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// normalizeFindings sorts findings and drops duplicate rule/line pairs so
// engine outputs compare deterministically.
func normalizeFindings(findings []Finding) []Finding {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Rule < findings[j].Rule
	})
	out := findings[:0]
	var prev Finding
	for i, f := range findings {
		if i > 0 && f == prev {
			continue
		}
		out = append(out, f)
		prev = f
	}
	return out
}
