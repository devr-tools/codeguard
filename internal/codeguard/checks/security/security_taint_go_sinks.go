package security

import (
	"go/ast"
	"strings"
)

var goQuerySinkMethods = map[string]int{
	"Query":           0,
	"QueryRow":        0,
	"Exec":            0,
	"Prepare":         0,
	"QueryContext":    1,
	"QueryRowContext": 1,
	"ExecContext":     1,
	"PrepareContext":  1,
}

var goFileSinkCallees = map[string]bool{
	"os.Open":         true,
	"os.OpenFile":     true,
	"os.Create":       true,
	"os.ReadFile":     true,
	"os.WriteFile":    true,
	"os.Remove":       true,
	"ioutil.ReadFile": true,
}

// goHTTPSinkArgIndex maps an outbound-HTTP callee to the argument index that
// carries the request URL. Tainted input reaching that argument is SSRF.
var goHTTPSinkArgIndex = map[string]int{
	"http.Get":                   0,
	"http.Head":                  0,
	"http.Post":                  0,
	"http.PostForm":              0,
	"http.NewRequest":            1,
	"http.NewRequestWithContext": 2,
}

// checkSinks inspects one call expression for taint sinks. Tainted values
// derived from parameters are recorded in the function summary instead of
// being reported, so callers decide whether the flow is dangerous.
func (s *goScope) checkSinks(call *ast.CallExpr, callee string, args []*taintTrace) {
	line := s.analyzer.line(call.Pos())
	switch {
	case callee == "exec.Command":
		s.reportFirstTainted(callee, line, args...)
	case callee == "exec.CommandContext" && len(args) > 1:
		s.reportFirstTainted(callee, line, args[1:]...)
	case goFileSinkCallees[callee] && len(args) > 0:
		s.reportSink(args[0], callee, line)
	case isGoHTTPSink(callee):
		if idx := goHTTPSinkArgIndex[callee]; idx < len(args) {
			s.reportSink(args[idx], callee, line)
		}
	default:
		s.checkMethodSinks(call, callee, args, line)
	}
}

func isGoHTTPSink(callee string) bool {
	_, ok := goHTTPSinkArgIndex[callee]
	return ok
}

func (s *goScope) checkMethodSinks(call *ast.CallExpr, callee string, args []*taintTrace, line int) {
	method := callee
	if dot := strings.LastIndexByte(callee, '.'); dot >= 0 {
		method = callee[dot+1:]
	}
	if receiver, selectedMethod, ok := selectorReceiverAndMethod(call.Fun); ok {
		if model, index, matches := s.analyzer.models.sinkModel(receiver, selectedMethod); matches && index < len(args) {
			s.reportSink(args[index].withSinkModel(model.name), callee, line)
			return
		}
	}
	if queryIdx, isQuery := goQuerySinkMethods[method]; isQuery && method != callee {
		if queryIdx < len(args) {
			s.reportSink(args[queryIdx], callee, line)
		}
		return
	}
	if method == "Parse" && s.isTemplateReceiver(call.Fun, callee) && len(args) > 0 {
		s.reportSink(args[0], callee, line)
	}
}

// isTemplateReceiver matches template.New(...).Parse and Parse calls on
// variables bound to template values.
func (s *goScope) isTemplateReceiver(fun ast.Expr, callee string) bool {
	if strings.HasPrefix(callee, "template.") || strings.Contains(callee, "template.New") {
		return true
	}
	if root, ok := rootIdent(fun); ok {
		return s.templateVars[root]
	}
	return false
}
