package security

import (
	"go/ast"
	"strings"
)

// goTaintPropagators are stdlib packages whose functions pass taint through.
var goTaintPropagators = []string{"fmt.", "strings.", "filepath.", "path.", "bytes.", "io.", "bufio."}

func (s *goScope) evalCall(call *ast.CallExpr) *goTaint {
	args := make([]*goTaint, len(call.Args))
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
	if taint := s.localCallTaint(call, callee, args); taint != nil {
		return taint
	}
	return s.propagatedCallTaint(call, callee, args)
}

// callSourceTaint recognizes calls that introduce fresh taint.
func (s *goScope) callSourceTaint(call *ast.CallExpr, callee string) *goTaint {
	if receiver, method, ok := selectorReceiverAndMethod(call.Fun); ok {
		if model := s.analyzer.models.sourceModel(receiver, method); model != "" {
			return s.sourceTaintWithModel(callee, call.Pos(), model)
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
func (s *goScope) localCallTaint(_ *ast.CallExpr, callee string, args []*goTaint) *goTaint {
	summary, known := s.analyzer.summaries[callee]
	if !known || summary == nil {
		return nil
	}
	s.applyParamSinks(callee, summary, args)
	if summary.returnTaint != nil {
		inner := summary.returnTaint
		return &goTaint{
			source:     inner.source,
			sourceLine: inner.sourceLine,
			chain:      append(append([]string{}, inner.chain...), callee+"()"),
			paramIndex: -1,
			model:      inner.model,
			sinkModel:  inner.sinkModel,
		}
	}
	for idx, taint := range args {
		if taint != nil && summary.paramsToReturn[idx] {
			return taint.extended(callee + "()")
		}
	}
	return nil
}

// applyParamSinks reports flows where a tainted argument reaches a sink
// inside a same-file callee.
func (s *goScope) applyParamSinks(callee string, summary *goFuncSummary, args []*goTaint) {
	for _, paramSink := range summary.paramsToSink {
		if paramSink.paramIndex >= len(args) || args[paramSink.paramIndex] == nil {
			continue
		}
		taint := args[paramSink.paramIndex].extended(callee + "()")
		s.reportSink(taint, paramSink.sink, paramSink.line)
	}
}

func (s *goScope) propagatedCallTaint(call *ast.CallExpr, callee string, args []*goTaint) *goTaint {
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
