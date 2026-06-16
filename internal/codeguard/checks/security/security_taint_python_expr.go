package security

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

var (
	pySanitizerCallPattern = regexp.MustCompile(`(?:^|[^\w.])((?:shlex\.quote|int|float)\s*\()`)
	pyInputSourcePattern   = regexp.MustCompile(`(?:^|[^\w.])input\s*\(`)
	pyEnvironSourcePattern = regexp.MustCompile(`(?:^|[^\w.])os\.environ\b`)
	pyArgvSourcePattern    = regexp.MustCompile(`(?:^|[^\w.])sys\.argv\b`)
	pyRequestSourcePattern = regexp.MustCompile(`(?:^|[^\w.])request\.(?:args|form|values|json|data|cookies|headers|GET|POST|FILES|query_params|get_json)\b`)
	pyIdentScanPattern     = regexp.MustCompile(`[A-Za-z_]\w*`)
)

// stripPySanitizers removes shlex.quote(...), int(...), and float(...)
// spans so sanitized values stop carrying taint.
func stripPySanitizers(text string) string {
	for {
		match := pySanitizerCallPattern.FindStringSubmatchIndex(text)
		if match == nil {
			return text
		}
		openParen := match[3] - 1
		closeParen := matchingParenOffset(text, openParen)
		if closeParen < 0 {
			return text[:match[2]] + text[match[3]:]
		}
		text = text[:match[2]] + text[closeParen+1:]
	}
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
func (s *pyScope) evalExpr(expr string, line int) *pyTaint {
	stripped := stripPySanitizers(expr)
	if taint := s.directSourceTaint(stripped, line); taint != nil {
		return taint
	}
	taint := s.localCallTaint(stripped, line)
	return preferPyTaint(taint, s.taintedIdentifier(stripped, line))
}

func (s *pyScope) directSourceTaint(stripped string, line int) *pyTaint {
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
	return &pyTaint{source: name, sourceLine: line, chain: []string{name}, paramIndex: -1}
}

func (s *pyScope) requestSourcesEnabled() bool {
	return s.analyzer.webRequest || s.requestParam
}

// localCallTaint applies same-file function summaries to calls inside the
// expression: tainted returns and tainted parameters that reach returns.
func (s *pyScope) localCallTaint(stripped string, line int) *pyTaint {
	for _, call := range support.ExtractCalls(stripped, line) {
		summary, known := s.analyzer.summaries[call.Callee]
		if !known || summary == nil {
			continue
		}
		if summary.returnTaint != nil {
			inner := summary.returnTaint
			return &pyTaint{
				source:     inner.source,
				sourceLine: inner.sourceLine,
				chain:      append(append([]string{}, inner.chain...), call.Callee+"()"),
				paramIndex: -1,
			}
		}
		for idx, arg := range call.Args {
			argTaint := s.taintedIdentifier(stripPySanitizers(arg), line)
			if argTaint != nil && summary.paramsToReturn[idx] {
				return argTaint.extended(call.Callee + "()")
			}
		}
	}
	return nil
}

// taintedIdentifier scans for identifiers bound to tainted values, skipping
// attribute accesses like obj.name.
func (s *pyScope) taintedIdentifier(stripped string, line int) *pyTaint {
	var found *pyTaint
	for _, match := range pyIdentScanPattern.FindAllStringIndex(stripped, -1) {
		if match[0] > 0 && stripped[match[0]-1] == '.' {
			continue
		}
		if taint, tracked := s.vars[stripped[match[0]:match[1]]]; tracked {
			found = preferPyTaint(found, taint)
		}
	}
	return found
}
