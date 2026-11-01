package security

import (
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// goTaintAnalyzer runs intra-file taint analysis with one summary-building
// pass followed by one reporting pass, giving call-site resolution for
// functions declared in the same file.
type goTaintAnalyzer struct {
	taintEngine
	fset      *token.FileSet
	functions map[string]*ast.FuncDecl
	models    goModelBindings
}

func goTaintFindings(env support.Context, file string, source string) []core.Finding {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, source, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	analyzer := &goTaintAnalyzer{
		taintEngine: newTaintEngine(env, file, taintRules{taintRuleID: "security.taint.go", ssrfRuleID: "security.ssrf.go", isSSRFSink: isGoHTTPSink}, true),
		fset:        fset,
		functions:   map[string]*ast.FuncDecl{},
		models:      newGoModelBindings(parsed),
	}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name != nil {
			analyzer.functions[fn.Name.Name] = fn
		}
	}
	analyzer.runSummaryPasses(analyzer.analyzeAll)
	return analyzer.findings
}

func (a *goTaintAnalyzer) analyzeAll(emit bool, next map[string]*funcSummary) {
	for name, fn := range a.functions {
		next[name] = a.analyzeFunction(fn, emit)
	}
}

func (a *goTaintAnalyzer) analyzeFunction(fn *ast.FuncDecl, emit bool) *funcSummary {
	scope := newGoScope(a, fn, emit)
	if fn.Body != nil {
		scope.walkStmts(fn.Body.List)
	}
	return scope.summary
}

func (a *goTaintAnalyzer) line(pos token.Pos) int {
	return a.fset.Position(pos).Line
}

// goScope is the per-function taint state.
type goScope struct {
	taintScope
	analyzer     *goTaintAnalyzer
	requestVars  map[string]bool
	stdinReaders map[string]bool
	templateVars map[string]bool
}

func newGoScope(a *goTaintAnalyzer, fn *ast.FuncDecl, emit bool) *goScope {
	scope := &goScope{
		taintScope:   newTaintScope(&a.taintEngine, emit),
		analyzer:     a,
		requestVars:  map[string]bool{},
		stdinReaders: map[string]bool{},
		templateVars: map[string]bool{},
	}
	scope.bindParams(fn)
	return scope
}

func (s *goScope) bindParams(fn *ast.FuncDecl) {
	if fn.Type.Params == nil {
		return
	}
	index := 0
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			s.analyzer.models.bindParam(name.Name, field.Type)
			if _, isRequest := s.analyzer.models.requests[name.Name]; isRequest {
				s.requestVars[name.Name] = true
			} else {
				s.vars[name.Name] = &taintTrace{
					source:     "parameter " + name.Name,
					sourceLine: s.analyzer.line(name.Pos()),
					chain:      []string{name.Name},
					paramIndex: index,
				}
			}
			index++
		}
	}
}
