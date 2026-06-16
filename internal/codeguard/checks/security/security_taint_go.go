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
	env       support.Context
	file      string
	fset      *token.FileSet
	functions map[string]*ast.FuncDecl
	summaries map[string]*goFuncSummary
	findings  []core.Finding
	seen      map[string]struct{}
}

func goTaintFindings(env support.Context, file string, source string) []core.Finding {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, source, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	analyzer := &goTaintAnalyzer{
		env:       env,
		file:      file,
		fset:      fset,
		functions: map[string]*ast.FuncDecl{},
		summaries: map[string]*goFuncSummary{},
		seen:      map[string]struct{}{},
	}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name != nil {
			analyzer.functions[fn.Name.Name] = fn
		}
	}
	analyzer.runPasses()
	return analyzer.findings
}

func (a *goTaintAnalyzer) runPasses() {
	for pass := 0; pass < 3; pass++ {
		emit := pass == 2
		next := map[string]*goFuncSummary{}
		for name, fn := range a.functions {
			next[name] = a.analyzeFunction(fn, emit)
		}
		a.summaries = next
	}
}

func (a *goTaintAnalyzer) analyzeFunction(fn *ast.FuncDecl, emit bool) *goFuncSummary {
	scope := newGoScope(a, fn, emit)
	if fn.Body != nil {
		scope.walkStmts(fn.Body.List)
	}
	return scope.summary
}

func (a *goTaintAnalyzer) line(pos token.Pos) int {
	return a.fset.Position(pos).Line
}

// emitFinding records one deduplicated source-to-sink finding.
func (a *goTaintAnalyzer) emitFinding(taint *goTaint, sink string, sinkLine int) {
	a.findings = appendTaintFinding(a.env, a.file, a.seen, a.findings, taintSinkInput{
		ruleID:     "security.taint.go",
		source:     taint.source,
		sourceLine: taint.sourceLine,
		chain:      taint.chain,
		sink:       sink,
		sinkLine:   sinkLine,
	})
}

// goScope is the per-function taint state.
type goScope struct {
	analyzer     *goTaintAnalyzer
	emit         bool
	vars         map[string]*goTaint
	requestVars  map[string]bool
	stdinReaders map[string]bool
	templateVars map[string]bool
	summary      *goFuncSummary
}

func newGoScope(a *goTaintAnalyzer, fn *ast.FuncDecl, emit bool) *goScope {
	scope := &goScope{
		analyzer:     a,
		emit:         emit,
		vars:         map[string]*goTaint{},
		requestVars:  map[string]bool{},
		stdinReaders: map[string]bool{},
		templateVars: map[string]bool{},
		summary:      &goFuncSummary{paramsToReturn: map[int]bool{}},
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
		isRequest := exprTypeText(field.Type) == "*http.Request"
		for _, name := range field.Names {
			if isRequest {
				s.requestVars[name.Name] = true
			} else {
				s.vars[name.Name] = &goTaint{
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
