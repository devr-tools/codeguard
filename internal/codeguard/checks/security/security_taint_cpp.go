package security

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// cppTaintAnalyzer uses the lightweight, comment/string-masked C-like parser.
// Two summary passes and a reporting pass resolve bounded same-file call paths.
type cppTaintAnalyzer struct {
	env       support.Context
	file      string
	parsed    *support.ParsedFile
	summaries map[string]*cppSummary
	findings  []core.Finding
	seen      map[string]struct{}
}

func cppTaintFindings(env support.Context, file string, source string) []core.Finding {
	analyzer := &cppTaintAnalyzer{
		env:       env,
		file:      file,
		parsed:    support.ParseCLike(source, support.CLikeCPP),
		summaries: map[string]*cppSummary{},
		seen:      map[string]struct{}{},
	}
	analyzer.runPasses()
	return analyzer.findings
}

func (a *cppTaintAnalyzer) runPasses() {
	for pass := 0; pass < 3; pass++ {
		emit := pass == 2
		next := map[string]*cppSummary{}
		for _, fn := range a.parsed.AllFunctions() {
			summary := a.analyzeScope(fn, emit)
			next[fn.Name] = summary
			if base := cppCalleeBase(fn.Name); base != fn.Name {
				if _, exists := next[base]; !exists {
					next[base] = summary
				}
			}
		}
		a.summaries = next
	}
}

func (a *cppTaintAnalyzer) analyzeScope(fn *support.ParsedFunction, emit bool) *cppSummary {
	scope := &cppScope{
		analyzer: a,
		fn:       fn,
		emit:     emit,
		vars:     map[string]*cppTaint{},
		summary:  &cppSummary{paramsToReturn: map[int]bool{}},
	}
	scope.bindParams()
	for _, statement := range fn.Statements {
		scope.processStatement(statement)
	}
	return scope.summary
}

type cppScope struct {
	analyzer *cppTaintAnalyzer
	fn       *support.ParsedFunction
	emit     bool
	vars     map[string]*cppTaint
	summary  *cppSummary
}

func (s *cppScope) bindParams() {
	for index, param := range s.fn.Params {
		taint := &cppTaint{
			source:     "parameter " + param.Name,
			sourceLine: s.fn.StartLine,
			chain:      []string{param.Name},
			paramIndex: index,
		}
		if cppCalleeBase(s.fn.Name) == "main" && (param.Name == "argv" || param.Name == "envp") {
			taint.source = "main parameter " + param.Name
			taint.paramIndex = -1
		}
		s.vars[param.Name] = taint
	}
}

func (s *cppScope) processStatement(statement support.ParsedStatement) {
	s.bindInputWrites(statement)
	s.checkStatementSinks(statement)
	s.applyAssignments(statement)
	trimmed := strings.TrimSpace(statement.Text)
	if rest, isReturn := strings.CutPrefix(trimmed, "return "); isReturn {
		s.recordReturn(strings.TrimSuffix(strings.TrimSpace(rest), ";"), statement.Line)
	}
}

func (s *cppScope) applyAssignments(statement support.ParsedStatement) {
	for _, assignment := range s.fn.Assignments {
		if assignment.Line == statement.Line {
			s.updateAssignedValue(assignment)
		}
	}
}

func (s *cppScope) updateAssignedValue(assignment support.ParsedAssignment) {
	if taint := s.evalExpr(assignment.Expr, assignment.Line); taint != nil {
		s.vars[assignment.Name] = taint.extended(assignment.Name)
		return
	}
	// Appending a constant does not make an existing tainted value safe.
	if !assignment.Augmented {
		delete(s.vars, assignment.Name)
	}
}

func (s *cppScope) recordReturn(expr string, line int) {
	taint := s.evalExpr(expr, line)
	switch {
	case taint == nil:
		// No summary effect for a return value proven untainted.
	case taint.paramIndex >= 0:
		s.summary.paramsToReturn[taint.paramIndex] = true
	case s.summary.returnTaint == nil:
		s.summary.returnTaint = taint
	}
}

func cppCalleeBase(callee string) string {
	for _, separator := range []string{"->", "::", "."} {
		if cut := strings.LastIndex(callee, separator); cut >= 0 {
			callee = callee[cut+len(separator):]
		}
	}
	return callee
}
