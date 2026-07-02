package treesitter

import (
	"fmt"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// pureGoEngine runs the rules on github.com/odvcencio/gotreesitter, a
// pure-Go (CGO_ENABLED=0 compatible) reimplementation of the tree-sitter
// runtime that loads the same grammar tables as upstream parser.c.
type pureGoEngine struct {
	lang      *gotreesitter.Language
	parser    *gotreesitter.Parser
	anyQuery  *gotreesitter.Query
	sinkQuery *gotreesitter.Query
}

func newPureGoEngine() (*pureGoEngine, error) {
	lang := grammars.TypescriptLanguage()
	if lang == nil {
		return nil, fmt.Errorf("gotreesitter: typescript grammar unavailable")
	}
	anyQuery, err := gotreesitter.NewQuery(explicitAnyQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("gotreesitter: compile explicit-any query: %w", err)
	}
	sinkQuery, err := gotreesitter.NewQuery(htmlSinkQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("gotreesitter: compile html-sink query: %w", err)
	}
	return &pureGoEngine{
		lang:      lang,
		parser:    gotreesitter.NewParser(lang),
		anyQuery:  anyQuery,
		sinkQuery: sinkQuery,
	}, nil
}

func (e *pureGoEngine) Name() string { return "gotreesitter (pure Go)" }

func (e *pureGoEngine) Scan(source []byte) ([]Finding, error) {
	tree, err := e.parse(source)
	if err != nil {
		return nil, err
	}
	return e.scanTree(tree, source), nil
}

func (e *pureGoEngine) parse(source []byte) (*gotreesitter.Tree, error) {
	tree, err := e.parser.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("gotreesitter: parse: %w", err)
	}
	return tree, nil
}

// scanTree evaluates both rule queries against an already-parsed tree. This
// is the path a corpus-level tree cache would hit for every rule after the
// first.
func (e *pureGoEngine) scanTree(tree *gotreesitter.Tree, source []byte) []Finding {
	root := tree.RootNode()
	findings := e.collectExplicitAny(root, source)
	findings = append(findings, e.collectHTMLSinks(root, source)...)
	return normalizeFindings(findings)
}

func (e *pureGoEngine) collectExplicitAny(root *gotreesitter.Node, source []byte) []Finding {
	findings := make([]Finding, 0, 4)
	cursor := e.anyQuery.Exec(root, e.lang, source)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			return findings
		}
		for _, capture := range match.Captures {
			if finding, hit := classifyExplicitAny(pureGoCaptured(capture, source)); hit {
				findings = append(findings, finding)
			}
		}
	}
}

func (e *pureGoEngine) collectHTMLSinks(root *gotreesitter.Node, source []byte) []Finding {
	findings := make([]Finding, 0, 4)
	cursor := e.sinkQuery.Exec(root, e.lang, source)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			return findings
		}
		objectText := ""
		for _, capture := range match.Captures {
			if strings.TrimPrefix(capture.Name, "@") == "sink.object" {
				objectText = capture.Node.Text(source)
			}
		}
		for _, capture := range match.Captures {
			if finding, hit := classifyHTMLSink(pureGoCaptured(capture, source), objectText); hit {
				findings = append(findings, finding)
			}
		}
	}
}

func pureGoCaptured(capture gotreesitter.QueryCapture, source []byte) capturedNode {
	return capturedNode{
		capture: strings.TrimPrefix(capture.Name, "@"),
		text:    capture.Node.Text(source),
		line:    int(capture.Node.StartPoint().Row) + 1,
	}
}

func init() {
	engine, err := newPureGoEngine()
	if err != nil {
		panic(err)
	}
	engines = append(engines, engine)
}
