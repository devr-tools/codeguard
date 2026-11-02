package security

import "github.com/devr-tools/codeguard/internal/codeguard/checks/support"

// stmtTaintLang is what a statement-based engine (Python, C++) supplies to
// the shared statement walker: how to evaluate an expression and a summarised
// call's argument, how to resolve a callee to a same-file summary, how to
// extract calls from text, and which calls are language sinks.
type stmtTaintLang interface {
	evalExpr(expr string, line int) *taintTrace
	callArgTaint(arg string, line int) *taintTrace
	lookupSummary(callee string) *funcSummary
	extractCalls(text string, line int) []support.ParsedCall
	checkCallSink(call support.ParsedCall)
}

// stmtTaintScope is the per-function state and propagation shared by the
// engines that walk support.ParsedFunction statements rather than an AST.
type stmtTaintScope struct {
	taintScope
	fn   *support.ParsedFunction
	lang stmtTaintLang
}

func newStmtTaintScope(engine *taintEngine, emit bool, fn *support.ParsedFunction, lang stmtTaintLang) stmtTaintScope {
	return stmtTaintScope{taintScope: newTaintScope(engine, emit), fn: fn, lang: lang}
}

// applyAssignments rebinds every variable assigned on the statement's line.
// An augmented assignment keeps existing taint: appending a constant does not
// make a tainted value safe.
func (s *stmtTaintScope) applyAssignments(statement support.ParsedStatement) {
	for _, assignment := range s.fn.Assignments {
		if assignment.Line == statement.Line {
			s.bind(assignment.Name, s.lang.evalExpr(assignment.Expr, assignment.Line), !assignment.Augmented)
		}
	}
}

// checkStatementSinks inspects every call in the statement for language
// sinks and for same-file callees whose parameters reach sinks.
func (s *stmtTaintScope) checkStatementSinks(statement support.ParsedStatement) {
	for _, call := range s.lang.extractCalls(statement.Text, statement.Line) {
		s.lang.checkCallSink(call)
		s.applyLocalParamSinks(call)
	}
}

func (s *stmtTaintScope) argTaint(call support.ParsedCall, index int) *taintTrace {
	if index < 0 || index >= len(call.Args) {
		return nil
	}
	return s.lang.evalExpr(call.Args[index], call.Line)
}

// applyLocalParamSinks reports tainted arguments flowing into same-file
// functions whose parameters reach sinks.
func (s *stmtTaintScope) applyLocalParamSinks(call support.ParsedCall) {
	summary := s.lang.lookupSummary(call.Callee)
	if summary == nil {
		return
	}
	s.applyParamSinks(call.Callee, summary, len(call.Args), func(index int) *taintTrace {
		return s.lang.evalExpr(call.Args[index], call.Line)
	})
}

// localCallTaint applies same-file function summaries to the calls inside an
// expression: tainted returns and tainted parameters that reach returns.
func (s *stmtTaintScope) localCallTaint(expr string, line int) *taintTrace {
	for _, call := range s.lang.extractCalls(expr, line) {
		summary := s.lang.lookupSummary(call.Callee)
		if summary == nil {
			continue
		}
		argTaint := func(index int) *taintTrace { return s.lang.callArgTaint(call.Args[index], line) }
		if taint := s.summaryCallTaint(call.Callee, summary, len(call.Args), argTaint); taint != nil {
			return taint
		}
	}
	return nil
}
