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

func (s *cppScope) checkCallSink(call support.ParsedCall) {
	base := cppCalleeBase(call.Callee)
	switch {
	case cppSimpleProcessSinks[base]:
		s.reportSink(s.argTaint(call, 0), call.Callee, call.Line)
	case base == "CreateProcess" || base == "CreateProcessA" || base == "CreateProcessW":
		s.reportFirstTainted(call.Callee, call.Line, s.argTaint(call, 0), s.argTaint(call, 1))
	case base == "ShellExecute" || base == "ShellExecuteA" || base == "ShellExecuteW":
		s.reportFirstTainted(call.Callee, call.Line, s.argTaint(call, 2), s.argTaint(call, 3))
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
