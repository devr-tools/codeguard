package security

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

var (
	cppSanitizerCallPattern = regexp.MustCompile(`(?:^|[^\w:])((?:(?:std::)?(?:stoi|stol|stoll|stoul|stoull)|escape_shell_arg|shell_escape|allowlisted_url|validated_url)\s*\()`)
	cppEnvSourcePattern     = regexp.MustCompile(`(?:^|[^\w:])((?:std::)?getenv)\s*\(`)
	cppIdentScanPattern     = regexp.MustCompile(`[A-Za-z_]\w*`)
	cppInputExtractPattern  = regexp.MustCompile(`\b(?:std\s*::\s*)?cin\s*>>\s*([A-Za-z_]\w*)`)
	cppRequestTypePattern   = regexp.MustCompile(`(?i)\b(?:basic_)?(?:http_?request|request)\b`)
	cppAccessorSpaceRemover = strings.NewReplacer(" ", "", "\t", "")
)

func stripCPPSanitizers(text string) string {
	return stripTaintSanitizerCalls(text, cppSanitizerCallPattern)
}

func (s *cppScope) evalExpr(expr string, line int) *taintTrace {
	stripped := stripCPPSanitizers(expr)
	if taint := s.directSourceTaint(stripped, line); taint != nil {
		return taint
	}
	taint := s.localCallTaint(stripped, line)
	return preferTaint(taint, s.taintedIdentifier(stripped))
}

func (s *cppScope) directSourceTaint(expr string, line int) *taintTrace {
	if match := cppEnvSourcePattern.FindStringSubmatch(expr); match != nil {
		return sourceTrace(match[1]+"()", line, "")
	}
	for _, param := range s.fn.Params {
		if !cppRequestTypePattern.MatchString(param.Type) || !cppRequestAccessor(expr, param.Name) {
			continue
		}
		return sourceTrace(param.Name+" request data", line, "")
	}
	return nil
}

func cppRequestAccessor(expr string, param string) bool {
	normalized := removeCPPAccessorWhitespace(expr)
	return cppReceiverAccessor(normalized, param+".") || cppReceiverAccessor(normalized, param+"->")
}

func removeCPPAccessorWhitespace(expr string) string {
	return cppAccessorSpaceRemover.Replace(expr)
}

func cppReceiverAccessor(expr string, receiver string) bool {
	for offset := 0; offset < len(expr); {
		idx := strings.Index(expr[offset:], receiver)
		if idx < 0 {
			return false
		}
		idx += offset
		if idx > 0 && isCPPIdentByte(expr[idx-1]) {
			offset = idx + len(receiver)
			continue
		}
		suffix := expr[idx+len(receiver):]
		for _, accessor := range []string{"getParameter", "getQueryParameter", "getHeader", "query", "param", "url_params.get"} {
			if strings.HasPrefix(suffix, accessor) && (len(suffix) == len(accessor) || !isCPPIdentByte(suffix[len(accessor)])) {
				return true
			}
		}
		offset = idx + len(receiver)
	}
	return false
}

func isCPPIdentByte(ch byte) bool {
	return ch == '_' || (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

// callArgTaint evaluates a summarised call's argument as a full expression.
func (s *cppScope) callArgTaint(arg string, line int) *taintTrace {
	return s.evalExpr(arg, line)
}

func (s *cppScope) extractCalls(text string, line int) []support.ParsedCall {
	return support.ExtractCLikeCalls(text, line)
}

func (s *cppScope) taintedIdentifier(expr string) *taintTrace {
	var found *taintTrace
	for _, name := range cppIdentScanPattern.FindAllString(expr, -1) {
		if taint, tracked := s.vars[name]; tracked {
			found = preferTaint(found, taint)
		}
	}
	return found
}

func (s *cppScope) bindInputWrites(statement support.ParsedStatement) {
	for _, match := range cppInputExtractPattern.FindAllStringSubmatch(statement.Text, -1) {
		s.bindConcreteSource(match[1], "std::cin", statement.Line)
	}
	for _, call := range support.ExtractCLikeCalls(statement.Text, statement.Line) {
		if cppCalleeBase(call.Callee) != "getline" || len(call.Args) < 2 {
			continue
		}
		input := strings.ReplaceAll(call.Args[0], " ", "")
		name := strings.TrimSpace(call.Args[1])
		identifier := cppIdentScanPattern.FindString(name)
		if (input == "std::cin" || input == "cin") && identifier == name {
			s.bindConcreteSource(name, "std::getline(std::cin)", call.Line)
		}
	}
}

func (s *cppScope) bindConcreteSource(name string, source string, line int) {
	s.vars[name] = &taintTrace{source: source, sourceLine: line, chain: []string{source, name}, paramIndex: -1}
}

func (s *cppScope) lookupSummary(callee string) *funcSummary {
	if summary := s.engine.summaries[callee]; summary != nil {
		return summary
	}
	return s.engine.summaries[cppCalleeBase(callee)]
}
