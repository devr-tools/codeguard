package security

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// cppTaintAnalyzer uses the lightweight, comment/string-masked C-like parser.
// Two summary passes and a reporting pass resolve bounded same-file call paths.
type cppTaintAnalyzer struct {
	taintEngine
	parsed *support.ParsedFile
}

func cppTaintFindings(env support.Context, file string, source string) []core.Finding {
	analyzer := &cppTaintAnalyzer{
		taintEngine: newTaintEngine(env, file, taintRules{taintRuleID: "security.taint.cpp", ssrfRuleID: "security.ssrf.cpp", isSSRFSink: isCPPSSRFSink}, false),
		parsed:      support.ParseCLike(source, support.CLikeCPP),
	}
	analyzer.runSummaryPasses(analyzer.analyzeAll)
	return analyzer.findings
}

func (a *cppTaintAnalyzer) analyzeAll(emit bool, next map[string]*funcSummary) {
	for _, fn := range a.parsed.AllFunctions() {
		summary := a.analyzeScope(fn, emit)
		next[fn.Name] = summary
		if base := cppCalleeBase(fn.Name); base != fn.Name {
			if _, exists := next[base]; !exists {
				next[base] = summary
			}
		}
	}
}

func (a *cppTaintAnalyzer) analyzeScope(fn *support.ParsedFunction, emit bool) *funcSummary {
	scope := &cppScope{}
	scope.stmtTaintScope = newStmtTaintScope(&a.taintEngine, emit, fn, scope)
	scope.bindParams()
	for _, statement := range fn.Statements {
		scope.processStatement(statement)
	}
	return scope.summary
}

type cppScope struct {
	stmtTaintScope
}

func (s *cppScope) bindParams() {
	for index, param := range s.fn.Params {
		taint := &taintTrace{
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
		s.recordReturn(s.evalExpr(strings.TrimSuffix(strings.TrimSpace(rest), ";"), statement.Line))
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
