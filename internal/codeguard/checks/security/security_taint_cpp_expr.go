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

func (s *cppScope) evalExpr(expr string, line int) *cppTaint {
	stripped := stripCPPSanitizers(expr)
	if taint := s.directSourceTaint(stripped, line); taint != nil {
		return taint
	}
	taint := s.localCallTaint(stripped, line)
	return preferCPPTaint(taint, s.taintedIdentifier(stripped))
}

func (s *cppScope) directSourceTaint(expr string, line int) *cppTaint {
	if match := cppEnvSourcePattern.FindStringSubmatch(expr); match != nil {
		return &cppTaint{source: match[1] + "()", sourceLine: line, chain: []string{match[1] + "()"}, paramIndex: -1}
	}
	for _, param := range s.fn.Params {
		if !cppRequestTypePattern.MatchString(param.Type) || !cppRequestAccessor(expr, param.Name) {
			continue
		}
		source := param.Name + " request data"
		return &cppTaint{source: source, sourceLine: line, chain: []string{source}, paramIndex: -1}
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

func (s *cppScope) localCallTaint(expr string, line int) *cppTaint {
	for _, call := range support.ExtractCLikeCalls(expr, line) {
		summary := s.lookupSummary(call.Callee)
		if summary == nil {
			continue
		}
		if summary.returnTaint != nil {
			inner := summary.returnTaint
			return &cppTaint{
				source:     inner.source,
				sourceLine: inner.sourceLine,
				chain:      append(append([]string{}, inner.chain...), call.Callee+"()"),
				paramIndex: -1,
			}
		}
		for index, arg := range call.Args {
			if !summary.paramsToReturn[index] {
				continue
			}
			if taint := s.evalExpr(arg, line); taint != nil {
				return taint.extended(call.Callee + "()")
			}
		}
	}
	return nil
}

func (s *cppScope) taintedIdentifier(expr string) *cppTaint {
	var found *cppTaint
	for _, name := range cppIdentScanPattern.FindAllString(expr, -1) {
		if taint, tracked := s.vars[name]; tracked {
			found = preferCPPTaint(found, taint)
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
	s.vars[name] = &cppTaint{source: source, sourceLine: line, chain: []string{source, name}, paramIndex: -1}
}

func (s *cppScope) lookupSummary(callee string) *cppSummary {
	if summary := s.analyzer.summaries[callee]; summary != nil {
		return summary
	}
	return s.analyzer.summaries[cppCalleeBase(callee)]
}
