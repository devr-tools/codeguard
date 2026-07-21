package security

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// pyTaintAnalyzer runs intra-file Python taint analysis with two summary
// passes followed by one reporting pass, resolving same-file call chains.
type pyTaintAnalyzer struct {
	env        support.Context
	file       string
	parsed     *support.ParsedFile
	summaries  map[string]*pySummary
	models     pyModelBindings
	webRequest bool
	findings   []core.Finding
	seen       map[string]struct{}
}

func pythonTaintFindings(env support.Context, file string, source string) []core.Finding {
	parsed := support.ParsePython(source)
	analyzer := &pyTaintAnalyzer{
		env:        env,
		file:       file,
		parsed:     parsed,
		summaries:  map[string]*pySummary{},
		models:     newPyModelBindings(parsed.Imports),
		webRequest: hasWebFrameworkImport(parsed.Imports),
		seen:       map[string]struct{}{},
	}
	analyzer.runPasses()
	return analyzer.findings
}

func hasWebFrameworkImport(imports []support.ParsedImport) bool {
	for _, imp := range imports {
		module := strings.ToLower(imp.Module)
		if strings.HasPrefix(module, "flask") || strings.HasPrefix(module, "django") || strings.HasPrefix(module, "fastapi") {
			return true
		}
	}
	return false
}

func (a *pyTaintAnalyzer) runPasses() {
	for pass := 0; pass < 3; pass++ {
		emit := pass == 2
		next := map[string]*pySummary{}
		for _, fn := range a.parsed.AllFunctions() {
			next[fn.Name] = a.analyzeScope(fn, emit, false)
		}
		a.summaries = next
	}
	a.analyzeScope(a.parsed.Module, true, true)
}

// analyzeScope walks one function or the module top level in order,
// tracking assignments and checking sinks.
func (a *pyTaintAnalyzer) analyzeScope(fn *support.ParsedFunction, emit bool, isModule bool) *pySummary {
	scope := &pyScope{
		analyzer:      a,
		fn:            fn,
		emit:          emit,
		vars:          map[string]*pyTaint{},
		requestModels: map[string]string{},
		summary:       &pySummary{paramsToReturn: map[int]bool{}},
	}
	scope.bindParams(isModule)
	for _, statement := range fn.Statements {
		scope.processStatement(statement)
	}
	return scope.summary
}

type pyScope struct {
	analyzer      *pyTaintAnalyzer
	fn            *support.ParsedFunction
	emit          bool
	vars          map[string]*pyTaint
	requestParam  bool
	requestModels map[string]string
	summary       *pySummary
}

// bindParams marks parameters as conditionally tainted. A leading self or
// cls receiver is skipped so call-site argument indexes line up.
func (s *pyScope) bindParams(isModule bool) {
	if isModule {
		return
	}
	index := 0
	for position, param := range s.fn.Params {
		if position == 0 && (param.Name == "self" || param.Name == "cls") {
			continue
		}
		if model := s.analyzer.models.requestModel(param); model != "" {
			s.requestParam = true
			s.requestModels[param.Name] = model
		}
		s.vars[param.Name] = &pyTaint{
			source:     "parameter " + param.Name,
			sourceLine: s.fn.StartLine,
			chain:      []string{param.Name},
			paramIndex: index,
		}
		index++
	}
}

func (s *pyScope) processStatement(statement support.ParsedStatement) {
	s.checkStatementSinks(statement)
	s.applyAssignments(statement)
	trimmed := strings.TrimSpace(statement.Text)
	if rest, isReturn := strings.CutPrefix(trimmed, "return "); isReturn {
		s.recordReturn(rest, statement.Line)
	}
}

func (s *pyScope) recordReturn(expr string, line int) {
	taint := s.evalExpr(expr, line)
	if taint == nil {
		return
	}
	if taint.paramIndex >= 0 {
		s.summary.paramsToReturn[taint.paramIndex] = true
		return
	}
	if s.summary.returnTaint == nil {
		s.summary.returnTaint = taint
	}
}

func (s *pyScope) applyAssignments(statement support.ParsedStatement) {
	for _, assignment := range s.fn.Assignments {
		if assignment.Line != statement.Line {
			continue
		}
		taint := s.evalExpr(assignment.Expr, assignment.Line)
		switch {
		case taint != nil:
			s.vars[assignment.Name] = taint.extended(assignment.Name)
		case assignment.Augmented:
			// keep any existing taint: += only appends
		default:
			delete(s.vars, assignment.Name)
		}
	}
}
