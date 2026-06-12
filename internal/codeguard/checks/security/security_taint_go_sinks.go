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

// checkSinks inspects one call expression for taint sinks. Tainted values
// derived from parameters are recorded in the function summary instead of
// being reported, so callers decide whether the flow is dangerous.
func (s *goScope) checkSinks(call *ast.CallExpr, callee string, args []*goTaint) {
	line := s.analyzer.line(call.Pos())
	switch {
	case callee == "exec.Command":
		s.reportFirstTainted(args, callee, line)
	case callee == "exec.CommandContext" && len(args) > 1:
		s.reportFirstTainted(args[1:], callee, line)
	case goFileSinkCallees[callee] && len(args) > 0:
		s.reportTainted(args[0], callee, line)
	default:
		s.checkMethodSinks(call, callee, args, line)
	}
}

func (s *goScope) checkMethodSinks(call *ast.CallExpr, callee string, args []*goTaint, line int) {
	method := callee
	if dot := strings.LastIndexByte(callee, '.'); dot >= 0 {
		method = callee[dot+1:]
	}
	if queryIdx, isQuery := goQuerySinkMethods[method]; isQuery && method != callee {
		if queryIdx < len(args) {
			s.reportTainted(args[queryIdx], callee, line)
		}
		return
	}
	if method == "Parse" && s.isTemplateReceiver(call.Fun, callee) && len(args) > 0 {
		s.reportTainted(args[0], callee, line)
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

func (s *goScope) reportFirstTainted(args []*goTaint, sink string, line int) {
	for _, taint := range args {
		if taint != nil {
			s.reportSink(taint, sink, line)
			return
		}
	}
}

func (s *goScope) reportTainted(taint *goTaint, sink string, line int) {
	if taint != nil {
		s.reportSink(taint, sink, line)
	}
}

// reportSink emits a finding for concrete sources and records a summary
// entry for parameter-conditional taint.
func (s *goScope) reportSink(taint *goTaint, sink string, line int) {
	if taint.paramIndex >= 0 {
		s.summary.paramsToSink = append(s.summary.paramsToSink, goParamSink{
			paramIndex: taint.paramIndex,
			sink:       sink,
			line:       line,
		})
		return
	}
	if s.emit {
		s.analyzer.emitFinding(taint, sink, line)
	}
}
