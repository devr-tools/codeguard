package security

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

var cppSimpleProcessSinks = map[string]bool{
	"system": true, "_wsystem": true, "popen": true, "_popen": true, "_wpopen": true,
	"execl": true, "execle": true, "execlp": true, "execv": true, "execve": true,
	"execvp": true, "execvpe": true, "WinExec": true,
}

func cppTaintRuleID(sink string) string {
	if isCPPSSRFSink(sink) {
		return "security.ssrf.cpp"
	}
	return "security.taint.cpp"
}

func (a *cppTaintAnalyzer) emitFinding(taint *cppTaint, sink string, sinkLine int) {
	a.findings = appendTaintFinding(a.env, a.file, a.seen, a.findings, taintSinkInput{
		ruleID:     cppTaintRuleID(sink),
		source:     taint.source,
		sourceLine: taint.sourceLine,
		chain:      taint.chain,
		sink:       sink,
		sinkLine:   sinkLine,
	})
}

func (s *cppScope) reportSink(taint *cppTaint, sink string, line int) {
	if taint == nil {
		return
	}
	if taint.paramIndex >= 0 {
		s.summary.paramsToSink = append(s.summary.paramsToSink, cppParamSink{
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

func (s *cppScope) checkStatementSinks(statement support.ParsedStatement) {
	for _, call := range support.ExtractCLikeCalls(statement.Text, statement.Line) {
		s.checkCallSink(call)
		s.applyLocalParamSinks(call)
	}
}

func (s *cppScope) checkCallSink(call support.ParsedCall) {
	base := cppCalleeBase(call.Callee)
	switch {
	case cppSimpleProcessSinks[base]:
		s.reportSink(s.argTaint(call, 0), call.Callee, call.Line)
	case base == "CreateProcess" || base == "CreateProcessA" || base == "CreateProcessW":
		s.reportFirstTainted(call, call.Callee, 0, 1)
	case base == "ShellExecute" || base == "ShellExecuteA" || base == "ShellExecuteW":
		s.reportFirstTainted(call, call.Callee, 2, 3)
	case call.Callee == "boost::process::system" || call.Callee == "boost::process::child":
		s.reportSink(s.argTaint(call, 0), call.Callee, call.Line)
	case base == "curl_easy_setopt":
		s.checkCurlURLSink(call)
	case isCPRRequestCall(call.Callee):
		s.reportSink(s.argTaint(call, 0), call.Callee, call.Line)
	case base == "SetUrl":
		s.reportSink(s.argTaint(call, 0), call.Callee, call.Line)
	case isBoostResolverCall(call.Callee):
		s.reportSink(s.argTaint(call, 0), call.Callee, call.Line)
	case strings.Contains(call.Callee, "web::http::client::http_client") || strings.Contains(call.Callee, "Poco::Net::HTTPClientSession"):
		s.reportSink(s.argTaint(call, 0), call.Callee, call.Line)
	}
}

func (s *cppScope) argTaint(call support.ParsedCall, index int) *cppTaint {
	if index < 0 || index >= len(call.Args) {
		return nil
	}
	return s.evalExpr(call.Args[index], call.Line)
}

func (s *cppScope) reportFirstTainted(call support.ParsedCall, sink string, indexes ...int) {
	for _, index := range indexes {
		if taint := s.argTaint(call, index); taint != nil {
			s.reportSink(taint, sink, call.Line)
			return
		}
	}
}

func (s *cppScope) applyLocalParamSinks(call support.ParsedCall) {
	summary := s.lookupSummary(call.Callee)
	if summary == nil {
		return
	}
	for _, paramSink := range summary.paramsToSink {
		if paramSink.paramIndex >= len(call.Args) {
			continue
		}
		taint := s.evalExpr(call.Args[paramSink.paramIndex], call.Line)
		if taint != nil {
			s.reportSink(taint.extended(call.Callee+"()"), paramSink.sink, paramSink.line)
		}
	}
}
