//go:build cgo

package treesitter

import (
	"fmt"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// cgoEngine runs the rules on the official CGo bindings
// (github.com/tree-sitter/go-tree-sitter) plus the official TypeScript
// grammar module. It exists in the spike as the correctness/performance
// reference for the native C runtime; it registers only when the build has
// cgo enabled, so `CGO_ENABLED=0 go test ./...` still exercises the pure-Go
// path alone.
type cgoEngine struct {
	lang         *tree_sitter.Language
	parser       *tree_sitter.Parser
	anyQuery     *tree_sitter.Query
	anyNames     []string
	sinkQuery    *tree_sitter.Query
	sinkNames    []string
	sinkObjIndex uint
}

func newCGoEngine() (*cgoEngine, error) {
	lang := tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(lang); err != nil {
		return nil, fmt.Errorf("cgo tree-sitter: set language: %w", err)
	}
	anyQuery, qerr := tree_sitter.NewQuery(lang, explicitAnyQuery)
	if qerr != nil {
		return nil, fmt.Errorf("cgo tree-sitter: compile explicit-any query: %w", qerr)
	}
	sinkQuery, qerr := tree_sitter.NewQuery(lang, htmlSinkQuery)
	if qerr != nil {
		return nil, fmt.Errorf("cgo tree-sitter: compile html-sink query: %w", qerr)
	}
	engine := &cgoEngine{
		lang:      lang,
		parser:    parser,
		anyQuery:  anyQuery,
		anyNames:  anyQuery.CaptureNames(),
		sinkQuery: sinkQuery,
		sinkNames: sinkQuery.CaptureNames(),
	}
	objIndex, ok := sinkQuery.CaptureIndexForName("sink.object")
	if !ok {
		return nil, fmt.Errorf("cgo tree-sitter: sink.object capture missing")
	}
	engine.sinkObjIndex = objIndex
	return engine, nil
}

func (e *cgoEngine) Name() string { return "official bindings (CGo)" }

func (e *cgoEngine) Scan(source []byte) ([]Finding, error) {
	tree, err := e.parse(source)
	if err != nil {
		return nil, err
	}
	defer tree.Close()
	return e.scanTree(tree, source), nil
}

func (e *cgoEngine) parse(source []byte) (*tree_sitter.Tree, error) {
	tree := e.parser.Parse(source, nil)
	if tree == nil {
		return nil, fmt.Errorf("cgo tree-sitter: parse returned no tree")
	}
	return tree, nil
}

func (e *cgoEngine) scanTree(tree *tree_sitter.Tree, source []byte) []Finding {
	findings := make([]Finding, 0, 8)
	root := tree.RootNode()

	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()
	matches := cursor.Matches(e.anyQuery, root, source)
	for match := matches.Next(); match != nil; match = matches.Next() {
		for _, capture := range match.Captures {
			if finding, hit := classifyExplicitAny(cgoCaptured(capture, e.anyNames, source)); hit {
				findings = append(findings, finding)
			}
		}
	}

	sinkCursor := tree_sitter.NewQueryCursor()
	defer sinkCursor.Close()
	matches = sinkCursor.Matches(e.sinkQuery, root, source)
	for match := matches.Next(); match != nil; match = matches.Next() {
		objectText := ""
		for _, capture := range match.Captures {
			if uint(capture.Index) == e.sinkObjIndex {
				objectText = capture.Node.Utf8Text(source)
			}
		}
		for _, capture := range match.Captures {
			if finding, hit := classifyHTMLSink(cgoCaptured(capture, e.sinkNames, source), objectText); hit {
				findings = append(findings, finding)
			}
		}
	}
	return normalizeFindings(findings)
}

func cgoCaptured(capture tree_sitter.QueryCapture, names []string, source []byte) capturedNode {
	return capturedNode{
		capture: names[capture.Index],
		text:    capture.Node.Utf8Text(source),
		line:    int(capture.Node.StartPosition().Row) + 1,
	}
}

func init() {
	engine, err := newCGoEngine()
	if err != nil {
		panic(err)
	}
	engines = append(engines, engine)
}
