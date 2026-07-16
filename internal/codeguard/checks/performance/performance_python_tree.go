package performance

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// pythonNPlusOneMessage is shared verbatim by the tree and regex paths so the
// tree-sitter upgrade is invisible in output except for precision (same rule
// ID, same level, same message).
const pythonNPlusOneMessage = "query or request call inside a loop suggests an N+1 pattern; batch the work or hoist the call out of the loop"

// pythonLoopQuery captures every for/while statement so call sites can be
// tested for loop containment by line span. The span includes the loop header
// line, matching the regex path (which flags `for row in cursor.execute(q):`
// on the header itself).
var pythonLoopQuery = support.CompileScriptQuery(`
(for_statement) @loop
(while_statement) @loop
`)

// pythonQueryCallQuery captures method-style calls (`receiver.method(...)`)
// with the receiver/method split, the syntactic counterpart of
// pythonQueryCallPattern: because only real call nodes match, query-shaped
// text inside comments and string literals — a false-positive class the regex
// path cannot avoid — never fires.
var pythonQueryCallQuery = support.CompileScriptQuery(`
(call
  function: (attribute
    object: (_) @call.object
    attribute: (identifier) @call.method))
`)

// pythonNPlusOneTreeFindings runs the tree-sitter path for
// performance.n-plus-one-query on one Python file. The boolean reports
// whether the tree path ran: false (parsers.treesitter off, grammar not
// embedded in this build, parse refusal, or query failure) means the caller
// must keep its regex scan, mirroring the fallback contract of
// security_typescript_tree.go.
func pythonNPlusOneTreeFindings(env support.Context, file string, source string) ([]core.Finding, bool) {
	tree := support.ScriptSyntaxTree(env, file, source)
	if tree == nil {
		return nil, false
	}
	loopHits, err := tree.Query(pythonLoopQuery)
	if err != nil {
		return nil, false
	}
	loops := pythonLoopSpans(loopHits)
	findings, ok := support.ScriptQueryFindings(env, file, tree, support.ScriptQuerySpec{
		Query:      pythonQueryCallQuery,
		RuleID:     "performance.n-plus-one-query",
		Level:      "warn",
		Message:    pythonNPlusOneMessage,
		Confidence: core.ConfidenceHigh,
		Classify: func(hit support.QueryHit) (int, bool) {
			return classifyPythonQueryCallHit(hit, loops)
		},
	})
	if !ok {
		return nil, false
	}
	return findings, true
}

// pythonLoopSpans reduces loop captures to inclusive [start, end] line spans.
func pythonLoopSpans(hits []support.QueryHit) [][2]int {
	spans := make([][2]int, 0, len(hits))
	for _, hit := range hits {
		for _, capture := range hit.Captures {
			if capture.Name == "loop" {
				spans = append(spans, [2]int{capture.Line, capture.EndLine})
			}
		}
	}
	return spans
}

// classifyPythonQueryCallHit keeps a call when it sits inside a for/while
// statement and its callee matches the query-call shapes of
// pythonQueryCallPattern: requests/httpx HTTP verbs, any `.execute(...)`
// (cursor objects go by many names), and `session.query(...)`.
func classifyPythonQueryCallHit(hit support.QueryHit, loops [][2]int) (int, bool) {
	object := hit.CaptureText("call.object")
	method := ""
	line := 0
	for _, capture := range hit.Captures {
		if capture.Name == "call.method" {
			method = capture.Text
			line = capture.Line
		}
	}
	if line == 0 || !lineInsideSpans(loops, line) {
		return 0, false
	}
	if !isPythonQueryCallee(object, method) {
		return 0, false
	}
	return line, true
}

func lineInsideSpans(spans [][2]int, line int) bool {
	for _, span := range spans {
		if line >= span[0] && line <= span[1] {
			return true
		}
	}
	return false
}

// isPythonQueryCallee mirrors pythonQueryCallPattern member by member so both
// paths agree on true positives; only the comment/string false-positive class
// differs.
func isPythonQueryCallee(object string, method string) bool {
	switch method {
	case "execute":
		return true
	case "get", "post", "put", "delete", "patch", "head":
		return pythonReceiverIs(object, "requests") || pythonReceiverIs(object, "httpx")
	case "query":
		return pythonReceiverIs(object, "session")
	default:
		return false
	}
}

// pythonReceiverIs matches the receiver exactly or as the last member of a
// dotted chain (`db.session` counts as `session`, matching the regex path's
// `\bsession\.query` word boundary).
func pythonReceiverIs(object string, name string) bool {
	return object == name || strings.HasSuffix(object, "."+name)
}
