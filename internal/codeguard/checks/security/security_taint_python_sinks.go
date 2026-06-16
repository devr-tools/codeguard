package security

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func (a *pyTaintAnalyzer) emitFinding(taint *pyTaint, sink string, sinkLine int) {
	a.findings = appendTaintFinding(a.env, a.file, a.seen, a.findings, taintSinkInput{
		ruleID:     "security.taint.python",
		source:     taint.source,
		sourceLine: taint.sourceLine,
		chain:      taint.chain,
		sink:       sink,
		sinkLine:   sinkLine,
	})
}

// reportSink emits concrete flows and records parameter-conditional flows
// in the function summary.
func (s *pyScope) reportSink(taint *pyTaint, sink string, line int) {
	if taint == nil {
		return
	}
	if taint.paramIndex >= 0 {
		s.summary.paramsToSink = append(s.summary.paramsToSink, pyParamSink{
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

var (
	pySubprocessPattern = regexp.MustCompile(`^subprocess\.(?:run|call|check_call|check_output|getoutput|getstatusoutput|Popen)$`)
	pyShellTruePattern  = regexp.MustCompile(`^shell\s*=\s*True$`)
)

// checkStatementSinks inspects every call in the statement for sinks.
func (s *pyScope) checkStatementSinks(statement support.ParsedStatement) {
	for _, call := range support.ExtractCalls(statement.Text, statement.Line) {
		s.checkCallSink(call)
		s.applyLocalParamSinks(call)
	}
}

func (s *pyScope) checkCallSink(call support.ParsedCall) {
	switch {
	case call.Callee == "os.system" || call.Callee == "os.popen" || call.Callee == "eval" || call.Callee == "exec":
		s.reportSink(s.argTaint(call, 0), call.Callee, call.Line)
	case pySubprocessPattern.MatchString(call.Callee):
		s.checkSubprocessSink(call)
	case strings.HasSuffix(call.Callee, ".execute") || strings.HasSuffix(call.Callee, ".executemany"):
		// only the query text is dangerous; parameterized args are safe
		s.reportSink(s.argTaint(call, 0), call.Callee, call.Line)
	}
}

// checkSubprocessSink flags subprocess calls that run through a shell or
// receive a tainted string command.
func (s *pyScope) checkSubprocessSink(call support.ParsedCall) {
	if len(call.Args) == 0 {
		return
	}
	shell := false
	for _, arg := range call.Args {
		if pyShellTruePattern.MatchString(strings.TrimSpace(arg)) {
			shell = true
		}
	}
	stringCommand := !strings.HasPrefix(strings.TrimSpace(call.Args[0]), "[")
	if !shell && !stringCommand {
		return
	}
	s.reportSink(s.argTaint(call, 0), call.Callee, call.Line)
}

func (s *pyScope) argTaint(call support.ParsedCall, index int) *pyTaint {
	if index >= len(call.Args) {
		return nil
	}
	return s.evalExpr(call.Args[index], call.Line)
}

// applyLocalParamSinks reports tainted arguments flowing into same-file
// functions whose parameters reach sinks.
func (s *pyScope) applyLocalParamSinks(call support.ParsedCall) {
	summary, known := s.analyzer.summaries[call.Callee]
	if !known || summary == nil {
		return
	}
	for _, paramSink := range summary.paramsToSink {
		if paramSink.paramIndex >= len(call.Args) {
			continue
		}
		taint := s.evalExpr(call.Args[paramSink.paramIndex], call.Line)
		if taint == nil {
			continue
		}
		s.reportSink(taint.extended(call.Callee+"()"), paramSink.sink, paramSink.line)
	}
}
