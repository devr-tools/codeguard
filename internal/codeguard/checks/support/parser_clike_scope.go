package support

import (
	"regexp"
	"strings"
)

var (
	tsDeclAssignPattern   = regexp.MustCompile(`^[ \t]*(?:export[ \t]+)?(?:const|let|var)[ \t]+([A-Za-z_$][\w$]*)[ \t]*(?::[^=\n]+?)?=[ \t]*([^=].*)$`)
	rustDeclAssignPattern = regexp.MustCompile(`^[ \t]*let[ \t]+(?:mut[ \t]+)?([A-Za-z_]\w*)[ \t]*(?::[^=\n]+?)?=[ \t]*(.+)$`)
	javaDeclAssignPattern = regexp.MustCompile(`^[ \t]*(?:final[ \t]+)?(?:[\w<>\[\],.?&]+(?:[ \t]+[\w<>\[\],.?&]+)*[ \t]+)?([A-Za-z_]\w*)[ \t]*=[ \t]*([^=].*)$`)
	cppDeclAssignPattern  = regexp.MustCompile(`^[ \t]*(?:constexpr[ \t]+|static[ \t]+|inline[ \t]+|const[ \t]+|volatile[ \t]+|mutable[ \t]+)*(?:[\w:<>[\],.?&*]+(?:[ \t]+[\w:<>[\],.?&*]+)*)[ \t]+([A-Za-z_]\w*)[ \t]*=[ \t]*([^=].*)$`)
	plainAssignPattern    = regexp.MustCompile(`^[ \t]*([A-Za-z_$][\w$]*)[ \t]*([-+*/%&|^]?)=[ \t]*([^=].*)$`)
	clikeCallPattern      = regexp.MustCompile(`([A-Za-z_$][\w$]*(?:(?:\.|::)[A-Za-z_$][\w$]*)*)[ \t]*\(`)
)

// clikeAssignments extracts declarations and reassignments from one masked
// body line.
func clikeAssignments(statement ParsedStatement, lang CLikeLanguage) []ParsedAssignment {
	text := strings.TrimSuffix(strings.TrimRight(statement.Text, " \t"), ";")
	if match := declAssignPatternFor(lang).FindStringSubmatch(text); match != nil {
		return []ParsedAssignment{{Name: match[1], Expr: strings.TrimSpace(match[2]), Line: statement.Line}}
	}
	if match := plainAssignPattern.FindStringSubmatch(text); match != nil && !isCLikeKeyword(match[1]) {
		return []ParsedAssignment{{Name: match[1], Expr: strings.TrimSpace(match[3]), Line: statement.Line, Augmented: match[2] != ""}}
	}
	return nil
}

func declAssignPatternFor(lang CLikeLanguage) *regexp.Regexp {
	switch lang {
	case CLikeRust:
		return rustDeclAssignPattern
	case CLikeJava:
		return javaDeclAssignPattern
	case CLikeCPP:
		return cppDeclAssignPattern
	default:
		return tsDeclAssignPattern
	}
}

// clikeCalls extracts call expressions from masked text.
func clikeCalls(text string, startLine int) []ParsedCall {
	calls := make([]ParsedCall, 0, 2)
	for _, match := range clikeCallPattern.FindAllStringSubmatchIndex(text, -1) {
		callee := text[match[2]:match[3]]
		base := callee
		if cut := strings.IndexAny(base, ".:"); cut >= 0 {
			base = base[:cut]
		}
		if isCLikeKeyword(base) {
			continue
		}
		args := splitTopLevelArgs(balancedSpan(text, match[1]-1))
		line := startLine + strings.Count(text[:match[2]], "\n")
		calls = append(calls, ParsedCall{Callee: callee, Args: args, Line: line})
	}
	return calls
}

func isCLikeKeyword(word string) bool {
	switch word {
	case "if", "else", "for", "while", "switch", "match", "catch", "return",
		"new", "typeof", "throw", "do", "in", "of", "loop", "fn", "function", "super":
		return true
	default:
		return false
	}
}

// parseCLikeParams splits a masked parameter list into named parameters.
func parseCLikeParams(paramText string, lang CLikeLanguage) []ParsedParam {
	params := make([]ParsedParam, 0, 4)
	for _, part := range splitTopLevelArgs(paramText) {
		part = strings.TrimSpace(part)
		if part == "" || part == "self" || strings.HasSuffix(part, " self") || strings.HasSuffix(part, "&self") || part == "&mut self" {
			continue
		}
		if param, ok := clikeParamFromPart(part, lang); ok {
			params = append(params, param)
		}
	}
	return params
}

func clikeParamFromPart(part string, lang CLikeLanguage) (ParsedParam, bool) {
	if eq := topLevelIndex(part, '='); eq >= 0 {
		part = strings.TrimSpace(part[:eq])
	}
	if lang == CLikeJava || lang == CLikeCPP {
		return javaParamFromPart(part)
	}
	name := part
	paramType := ""
	if colon := topLevelIndex(part, ':'); colon >= 0 {
		name = strings.TrimSpace(part[:colon])
		paramType = strings.TrimSpace(part[colon+1:])
	}
	name = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(name, "..."), "mut "))
	if !clikeIdentPattern.MatchString(name) {
		return ParsedParam{}, false
	}
	return ParsedParam{Name: name, Type: paramType}, true
}

func javaParamFromPart(part string) (ParsedParam, bool) {
	fields := strings.Fields(part)
	if len(fields) < 2 {
		return ParsedParam{}, false
	}
	name := fields[len(fields)-1]
	if !clikeIdentPattern.MatchString(name) {
		return ParsedParam{}, false
	}
	return ParsedParam{Name: name, Type: strings.Join(fields[:len(fields)-1], " ")}, true
}

var clikeIdentPattern = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)
