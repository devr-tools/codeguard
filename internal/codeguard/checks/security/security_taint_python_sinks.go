package security

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

// pyHTTPSinkArgIndex maps an outbound-HTTP callee to the argument index that
// carries the request URL. Tainted input reaching that argument is SSRF.
var pyHTTPSinkArgIndex = map[string]int{
	"requests.get":           0,
	"requests.post":          0,
	"requests.put":           0,
	"requests.delete":        0,
	"requests.patch":         0,
	"requests.head":          0,
	"requests.options":       0,
	"requests.request":       1,
	"urllib.request.urlopen": 0,
	"urllib.request.Request": 0,
	"urllib.urlopen":         0,
	"urllib2.urlopen":        0,
	"httpx.get":              0,
	"httpx.post":             0,
	"httpx.put":              0,
	"httpx.patch":            0,
	"httpx.delete":           0,
	"httpx.head":             0,
	"httpx.request":          1,
	"httpx.stream":           1,
}

func isPyHTTPSink(callee string) bool {
	_, ok := pyHTTPSinkArgIndex[callee]
	return ok
}

var (
	pySubprocessPattern = regexp.MustCompile(`^subprocess\.(?:run|call|check_call|check_output|getoutput|getstatusoutput|Popen)$`)
	pyShellTruePattern  = regexp.MustCompile(`^shell\s*=\s*True$`)
)

func (s *pyScope) checkCallSink(call support.ParsedCall) {
	call.Callee = s.analyzer.models.canonicalCallee(call.Callee)
	switch {
	case call.Callee == "os.system" || call.Callee == "os.popen" || call.Callee == "eval" || call.Callee == "exec":
		s.reportSink(s.argTaint(call, 0), call.Callee, call.Line)
	case pySubprocessPattern.MatchString(call.Callee):
		s.checkSubprocessSink(call)
	case strings.HasSuffix(call.Callee, ".execute") || strings.HasSuffix(call.Callee, ".executemany"):
		// only the query text is dangerous; parameterized args are safe
		s.reportSink(s.argTaint(call, 0), call.Callee, call.Line)
	case s.analyzer.models.djangoRawSink(call.Callee):
		s.reportSink(s.argTaint(call, 0).withSinkModel("django-orm-raw-sql"), call.Callee, call.Line)
	case isPyHTTPSink(call.Callee):
		s.reportSink(s.argTaint(call, pyHTTPSinkArgIndex[call.Callee]), call.Callee, call.Line)
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
