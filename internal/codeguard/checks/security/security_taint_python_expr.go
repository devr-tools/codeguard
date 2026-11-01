package security

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

var (
	pySanitizerCallPattern        = regexp.MustCompile(`(?:^|[^\w.])((?:shlex\.quote|int|float)\s*\()`)
	pyInputSourcePattern          = regexp.MustCompile(`(?:^|[^\w.])input\s*\(`)
	pyEnvironSourcePattern        = regexp.MustCompile(`(?:^|[^\w.])os\.environ\b`)
	pyArgvSourcePattern           = regexp.MustCompile(`(?:^|[^\w.])sys\.argv\b`)
	pyRequestSourcePattern        = regexp.MustCompile(`(?:^|[^\w.])(?:flask\.)?request\.(?:args|form|values|json|data|cookies|headers|GET|POST|FILES|query_params|get_json)\b`)
	pyDjangoRequestSourcePattern  = regexp.MustCompile(`(?:^|[^\w.])([A-Za-z_]\w*)\.(?:GET|POST|FILES|COOKIES|META|body)\b`)
	pyFastAPIRequestSourcePattern = regexp.MustCompile(`(?:^|[^\w.])([A-Za-z_]\w*)\.(?:query_params|path_params|headers|cookies|body|json|form)\b`)
	pyIdentScanPattern            = regexp.MustCompile(`[A-Za-z_]\w*`)
)

// stripPySanitizers removes shlex.quote(...), int(...), and float(...)
// spans so sanitized values stop carrying taint.
func stripPySanitizers(text string) string {
	return stripTaintSanitizerCalls(text, pySanitizerCallPattern)
}

func matchingParenOffset(text string, open int) int {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// evalExpr computes the taint of one masked Python expression.
func (s *pyScope) evalExpr(expr string, line int) *taintTrace {
	stripped := stripPySanitizers(expr)
	if taint := s.directSourceTaint(stripped, line); taint != nil {
		return taint
	}
	taint := s.localCallTaint(stripped, line)
	return preferTaint(taint, s.taintedIdentifier(stripped, line))
}

func (s *pyScope) directSourceTaint(stripped string, line int) *taintTrace {
	if taint := s.frameworkRequestSource(stripped, line); taint != nil {
		return taint
	}
	name := ""
	switch {
	case pyInputSourcePattern.MatchString(stripped):
		name = "input()"
	case pyEnvironSourcePattern.MatchString(stripped):
		name = "os.environ"
	case pyArgvSourcePattern.MatchString(stripped):
		name = "sys.argv"
	case s.requestSourcesEnabled() && pyRequestSourcePattern.MatchString(stripped):
		name = pyRequestSourcePattern.FindString(stripped)
		name = strings.TrimLeft(name, " \t=+,([{")
	}
	if name == "" {
		return nil
	}
	return sourceTrace(name, line, "")
}

func (s *pyScope) frameworkRequestSource(stripped string, line int) *taintTrace {
	for _, candidate := range []struct {
		model   string
		pattern *regexp.Regexp
	}{{"django", pyDjangoRequestSourcePattern}, {"fastapi", pyFastAPIRequestSourcePattern}} {
		match := candidate.pattern.FindStringSubmatch(stripped)
		if len(match) < 2 || s.requestModels[match[1]] != candidate.model {
			continue
		}
		name := strings.TrimLeft(match[0], " \t=+,([{")
		return sourceTrace(name, line, candidate.model)
	}
	return nil
}

func (s *pyScope) requestSourcesEnabled() bool {
	return s.analyzer.webRequest || s.requestParam
}

// callArgTaint resolves a summarised call's argument from tracked
// identifiers only; nested calls inside the argument are not re-evaluated.
func (s *pyScope) callArgTaint(arg string, line int) *taintTrace {
	return s.taintedIdentifier(stripPySanitizers(arg), line)
}

func (s *pyScope) extractCalls(text string, line int) []support.ParsedCall {
	return support.ExtractCallsPositional(text, line)
}

// lookupSummary resolves a callee by its exact same-file function name.
func (s *pyScope) lookupSummary(callee string) *funcSummary {
	return s.engine.summaries[callee]
}

// taintedIdentifier scans for identifiers bound to tainted values, skipping
// attribute accesses like obj.name.
func (s *pyScope) taintedIdentifier(stripped string, _ int) *taintTrace {
	var found *taintTrace
	for _, match := range pyIdentScanPattern.FindAllStringIndex(stripped, -1) {
		if match[0] > 0 && stripped[match[0]-1] == '.' {
			continue
		}
		if taint, tracked := s.vars[stripped[match[0]:match[1]]]; tracked {
			found = preferTaint(found, taint)
		}
	}
	return found
}
