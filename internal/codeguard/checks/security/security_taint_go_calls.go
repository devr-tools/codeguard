package security

import (
	"go/ast"
	"strings"
)

// goTaintPropagators are stdlib packages whose functions pass taint through.
var goTaintPropagators = []string{"fmt.", "strings.", "filepath.", "path.", "bytes.", "io.", "bufio."}

func (s *goScope) evalCall(call *ast.CallExpr) *taintTrace {
	s.evalChainedReceiver(call.Fun)
	args := make([]*taintTrace, len(call.Args))
	for idx, arg := range call.Args {
		args[idx] = s.evalExpr(arg)
	}
	callee := exprTypeText(call.Fun)
	s.checkSinks(call, callee, args)
	if taint := s.callSourceTaint(call, callee); taint != nil {
		return taint
	}
	if strings.HasPrefix(callee, "strconv.") {
		return nil // parsed values are sanitized
	}
	if taint := s.localCallTaint(callee, args); taint != nil {
		return taint
	}
	return s.propagatedCallTaint(call, callee, args)
}

// evalChainedReceiver exists so exec.Command(arg).Run() is held to the same
// standard as cmd := exec.Command(arg); cmd.Run(). exprTypeText flattens the
// receiver call into the outer callee text, so its arguments would otherwise
// never reach checkSinks. The result is discarded on purpose: the outer
// call's taint stays governed by the propagation rules, so this cannot widen
// what a chained call returns.
func (s *goScope) evalChainedReceiver(fun ast.Expr) {
	selector, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	receiver := selector.X
	for {
		paren, isParen := receiver.(*ast.ParenExpr)
		if !isParen {
			break
		}
		receiver = paren.X
	}
	if inner, isCall := receiver.(*ast.CallExpr); isCall {
		s.evalCall(inner)
		return
	}
	if inner, isSelector := receiver.(*ast.SelectorExpr); isSelector {
		s.evalChainedReceiver(inner)
	}
}

// callSourceTaint recognizes calls that introduce fresh taint.
func (s *goScope) callSourceTaint(call *ast.CallExpr, callee string) *taintTrace {
	if receiver, method, ok := selectorReceiverAndMethod(call.Fun); ok {
		if model := s.analyzer.models.sourceModel(receiver, method); model != "" {
			return sourceTrace(callee, s.analyzer.line(call.Pos()), model)
		}
	}
	root := callee
	if dot := strings.IndexByte(root, '.'); dot >= 0 {
		root = root[:dot]
	}
	switch {
	case callee == "os.Getenv":
		return s.sourceTaint("os.Getenv", call.Pos())
	case s.requestVars[root] && root != callee:
		return s.sourceTaint(callee, call.Pos())
	case s.stdinReaders[root] && root != callee:
		return s.sourceTaint(callee+" (stdin)", call.Pos())
	default:
		return nil
	}
}

func selectorReceiverAndMethod(expr ast.Expr) (string, string, bool) {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	receiver, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	return receiver.Name, selector.Sel.Name, true
}

// localCallTaint applies same-file function summaries at call sites.
func (s *goScope) localCallTaint(callee string, args []*taintTrace) *taintTrace {
	summary := s.engine.summaries[callee]
	if summary == nil {
		return nil
	}
	argTaint := func(index int) *taintTrace { return args[index] }
	s.applyParamSinks(callee, summary, len(args), argTaint)
	return s.summaryCallTaint(callee, summary, len(args), argTaint)
}

func (s *goScope) propagatedCallTaint(call *ast.CallExpr, callee string, args []*taintTrace) *taintTrace {
	for _, prefix := range goTaintPropagators {
		if !strings.HasPrefix(callee, prefix) {
			continue
		}
		for _, taint := range args {
			if taint != nil {
				return taint.extended(callee)
			}
		}
		return nil
	}
	if root, ok := rootIdent(call.Fun); ok {
		return s.vars[root] // method call on a tainted receiver
	}
	return nil
}
