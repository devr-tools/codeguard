package support

import (
	"fmt"
	"strings"
	"sync"

	"github.com/odvcencio/gotreesitter"
)

// QueryCapture is the engine-independent view of one named capture inside a
// query match: capture name (without the leading @), node text, and the
// 1-based start/end lines of the node.
type QueryCapture struct {
	Name    string
	Text    string
	Line    int
	EndLine int
}

// QueryHit is one query match: the grouped captures of a single pattern
// occurrence, in capture order.
type QueryHit struct {
	Captures []QueryCapture
}

// CaptureText returns the text of the first capture with the given name, or
// "" when the pattern did not bind it.
func (h QueryHit) CaptureText(name string) string {
	for _, capture := range h.Captures {
		if capture.Name == name {
			return capture.Text
		}
	}
	return ""
}

// CompiledQuery is a tree-sitter query source compiled lazily once per
// grammar (queries are grammar-specific objects; the same source usually
// compiles against typescript, tsx, and javascript alike as the three share
// node names). Construct package-level instances with CompileScriptQuery and
// reuse them across files; all methods are safe for concurrent use.
type CompiledQuery struct {
	source string
	mu     sync.Mutex
	byLang map[ScriptLanguage]*compiledQueryEntry
}

type compiledQueryEntry struct {
	query *gotreesitter.Query
	err   error
}

// CompileScriptQuery wraps a query source for per-language lazy compilation.
// Compilation errors surface from SyntaxTree.Query, letting callers fall
// back to their regex path.
func CompileScriptQuery(source string) *CompiledQuery {
	return &CompiledQuery{source: source, byLang: map[ScriptLanguage]*compiledQueryEntry{}}
}

func (q *CompiledQuery) forLanguage(lang ScriptLanguage, language *gotreesitter.Language) (*gotreesitter.Query, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry, ok := q.byLang[lang]
	if !ok {
		query, err := gotreesitter.NewQuery(q.source, language)
		if err != nil {
			err = fmt.Errorf("compile script query for %s: %w", lang, err)
		}
		entry = &compiledQueryEntry{query: query, err: err}
		q.byLang[lang] = entry
	}
	return entry.query, entry.err
}

// Query evaluates a compiled query against the tree and returns one hit per
// match. Errors are compile errors for this tree's grammar; callers treat
// them as "tree path unavailable" and fall back to their regex scan.
func (t *SyntaxTree) Query(q *CompiledQuery) ([]QueryHit, error) {
	query, err := q.forLanguage(t.lang, t.language)
	if err != nil {
		return nil, err
	}
	cursor := query.Exec(t.tree.RootNode(), t.language, t.source)
	hits := make([]QueryHit, 0, 8)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			return hits, nil
		}
		hit := QueryHit{Captures: make([]QueryCapture, 0, len(match.Captures))}
		for _, capture := range match.Captures {
			hit.Captures = append(hit.Captures, QueryCapture{
				Name:    strings.TrimPrefix(capture.Name, "@"),
				Text:    capture.Node.Text(t.source),
				Line:    int(capture.Node.StartPoint().Row) + 1,
				EndLine: int(capture.Node.EndPoint().Row) + 1,
			})
		}
		hits = append(hits, hit)
	}
}
