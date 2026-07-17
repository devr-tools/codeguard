package security

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func isCPPSSRFSink(sink string) bool {
	return strings.HasPrefix(sink, "curl_easy_setopt[") || strings.HasPrefix(sink, "cpr::") ||
		strings.HasSuffix(sink, "SetUrl") || strings.HasSuffix(sink, ".resolve") ||
		strings.HasSuffix(sink, "->resolve") || strings.Contains(sink, "http_client") ||
		strings.Contains(sink, "HTTPClientSession")
}

func (s *cppScope) checkCurlURLSink(call support.ParsedCall) {
	if len(call.Args) < 3 || strings.TrimSpace(call.Args[1]) != "CURLOPT_URL" {
		return
	}
	s.reportSink(s.argTaint(call, 2), "curl_easy_setopt[CURLOPT_URL]", call.Line)
}

func isCPRRequestCall(callee string) bool {
	if !strings.HasPrefix(callee, "cpr::") {
		return false
	}
	switch cppCalleeBase(callee) {
	case "Get", "Post", "Put", "Delete", "Patch", "Head", "Options", "Download":
		return true
	default:
		return false
	}
}

func isBoostResolverCall(callee string) bool {
	if !strings.HasSuffix(callee, ".resolve") && !strings.HasSuffix(callee, "->resolve") && !strings.HasSuffix(callee, "::resolve") {
		return false
	}
	return strings.Contains(strings.ToLower(callee), "resolver")
}
