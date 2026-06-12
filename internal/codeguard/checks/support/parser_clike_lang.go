package support

import "regexp"

var (
	tsFunctionHead = regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?(?:default[ \t]+)?(?:async[ \t]+)?function[ \t]*\*?[ \t]*([A-Za-z_$][\w$]*)[ \t]*(?:<[^>\n]*>)?[ \t]*\(`)
	tsArrowHead    = regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?(?:const|let|var)[ \t]+([A-Za-z_$][\w$]*)[^=\n]*=[ \t]*(?:async[ \t]*)?\(`)
	tsMethodHead   = regexp.MustCompile(`(?m)^[ \t]*(?:(?:public|private|protected|static|readonly|async|override)[ \t]+)*([A-Za-z_$][\w$]*)[ \t]*(?:<[^>\n]*>)?[ \t]*\(`)
	javaMethodHead = regexp.MustCompile(`(?m)^[ \t]*(?:@\w+(?:\([^)\n]*\))?[ \t\n]*)*(?:(?:public|protected|private|static|final|abstract|synchronized|native|default|strictfp)[ \t]+)+[\w<>\[\],.?& \t]+?[ \t]([A-Za-z_]\w*)[ \t]*\(`)
	rustFnHead     = regexp.MustCompile(`(?m)^[ \t]*(?:pub(?:\([^)\n]*\))?[ \t]+)?(?:const[ \t]+)?(?:async[ \t]+)?(?:unsafe[ \t]+)?(?:extern[ \t]+\S+[ \t]+)?fn[ \t]+([A-Za-z_]\w*)`)
)

func clikeFunctionSpans(masked string, lang CLikeLanguage) []clikeSpan {
	switch lang {
	case CLikeJava:
		return headSpans(masked, javaMethodHead, nil, false)
	case CLikeRust:
		return rustSpans(masked)
	default:
		return typeScriptSpans(masked)
	}
}

// headSpans resolves regex head matches into full function spans. The regex
// must end at the open paren; reject filters out non-function names.
func headSpans(masked string, head *regexp.Regexp, reject func(string) bool, arrow bool) []clikeSpan {
	spans := make([]clikeSpan, 0, 8)
	for _, match := range head.FindAllStringSubmatchIndex(masked, -1) {
		name := masked[match[2]:match[3]]
		if reject != nil && reject(name) {
			continue
		}
		span, ok := resolveSpan(masked, match[0], match[1]-1, arrow)
		if !ok {
			continue
		}
		span.name = name
		spans = append(spans, span)
	}
	return spans
}

// resolveSpan completes a span from the signature's open paren by matching
// the parameter list and locating the body braces.
func resolveSpan(masked string, start int, paramsOpen int, arrow bool) (clikeSpan, bool) {
	span := clikeSpan{start: start, paramsOpen: paramsOpen}
	paramsClose := matchBracketOffset(masked, paramsOpen)
	if paramsClose < 0 {
		return span, false
	}
	if arrow {
		return resolveArrowSpan(masked, span, paramsClose)
	}
	span.bodyOpen = findBodyOpen(masked, paramsClose+1, false)
	if span.bodyOpen < 0 {
		return span, false
	}
	span.bodyEnd = matchBracketOffset(masked, span.bodyOpen)
	return span, span.bodyEnd > span.bodyOpen
}

// resolveArrowSpan requires an `=>` after the parameter list and supports
// both block bodies and expression bodies.
func resolveArrowSpan(masked string, span clikeSpan, paramsClose int) (clikeSpan, bool) {
	arrowAt := indexOfArrow(masked, paramsClose+1)
	if arrowAt < 0 {
		return span, false
	}
	rest := arrowAt + 2
	for rest < len(masked) && (masked[rest] == ' ' || masked[rest] == '\t' || masked[rest] == '\n') {
		rest++
	}
	if rest < len(masked) && masked[rest] == '{' {
		span.bodyOpen = rest
		span.bodyEnd = matchBracketOffset(masked, rest)
		return span, span.bodyEnd > span.bodyOpen
	}
	span.bodyOpen = -1
	span.bodyEnd = expressionEnd(masked, rest)
	return span, true
}

func indexOfArrow(masked string, offset int) int {
	depth := 0
	for i := offset; i < len(masked); i++ {
		switch masked[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ';':
			if depth <= 0 {
				return -1
			}
		case '=':
			if depth == 0 && i+1 < len(masked) && masked[i+1] == '>' {
				return i
			}
		}
	}
	return -1
}

// expressionEnd finds the end of an expression-bodied arrow function.
func expressionEnd(masked string, offset int) int {
	depth := 0
	for i := offset; i < len(masked); i++ {
		switch masked[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth < 0 {
				return i
			}
		case ';', '\n':
			if depth <= 0 {
				return i
			}
		}
	}
	return len(masked) - 1
}

func typeScriptSpans(masked string) []clikeSpan {
	spans := headSpans(masked, tsFunctionHead, nil, false)
	spans = append(spans, headSpans(masked, tsArrowHead, nil, true)...)
	spans = append(spans, headSpans(masked, tsMethodHead, isTypeScriptNonMethodName, false)...)
	return dedupeSpans(spans)
}

func isTypeScriptNonMethodName(name string) bool {
	switch name {
	case "if", "for", "while", "switch", "catch", "constructor", "function",
		"return", "new", "typeof", "await", "do", "else", "case", "throw", "super", "in", "of":
		return true
	default:
		return false
	}
}

// dedupeSpans drops spans whose parameter list was already claimed.
func dedupeSpans(spans []clikeSpan) []clikeSpan {
	seen := make(map[int]struct{}, len(spans))
	out := make([]clikeSpan, 0, len(spans))
	for _, span := range spans {
		if _, dup := seen[span.paramsOpen]; dup {
			continue
		}
		seen[span.paramsOpen] = struct{}{}
		out = append(out, span)
	}
	return out
}
