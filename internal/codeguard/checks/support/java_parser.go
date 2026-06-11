package support

var javaParserControlWords = map[string]struct{}{
	"if": {}, "for": {}, "while": {}, "switch": {}, "catch": {}, "return": {}, "new": {}, "throw": {}, "else": {}, "do": {}, "try": {}, "synchronized": {},
}

func ParseJavaFunctions(source string) []ParsedFunction {
	tokens := tokenizeCLikeSource(source, false)
	functions := make([]ParsedFunction, 0)
	for idx := 0; idx < len(tokens); idx++ {
		if tokens[idx].text != "{" {
			continue
		}
		headerStart := javaHeaderStart(tokens, idx)
		if headerStart < 0 || headerStart >= idx {
			continue
		}
		nameIdx, paramStart, paramEnd, ok := javaMethodSignature(tokens, headerStart, idx)
		if !ok {
			continue
		}
		bodyEnd := findMatchingToken(tokens, idx, "{", "}")
		if bodyEnd < 0 {
			continue
		}
		functions = append(functions, ParsedFunction{
			Name:       tokens[nameIdx].text,
			StartLine:  tokens[headerStart].line,
			EndLine:    tokens[bodyEnd].line,
			Parameters: source[tokens[paramStart].end:tokens[paramEnd].start],
			Body:       source[tokens[idx].end:tokens[bodyEnd].start],
		})
	}
	return functions
}

func javaHeaderStart(tokens []parserToken, braceIdx int) int {
	for idx := braceIdx - 1; idx >= 0; idx-- {
		switch tokens[idx].text {
		case ";", "{", "}":
			return idx + 1
		}
	}
	return 0
}

func javaMethodSignature(tokens []parserToken, start int, bodyStart int) (int, int, int, bool) {
	nameIdx := -1
	paramStart := -1
	paramEnd := -1
	for idx := start; idx < bodyStart; idx++ {
		if tokens[idx].text != "(" || idx == start {
			continue
		}
		candidateName := idx - 1
		if !isParserIdentifier(tokens[candidateName].text) {
			continue
		}
		if _, blocked := javaParserControlWords[tokens[candidateName].text]; blocked {
			continue
		}
		end := findMatchingToken(tokens, idx, "(", ")")
		if end < 0 || end >= bodyStart {
			continue
		}
		if !javaLooksLikeMethodHeader(tokens, start, candidateName, end, bodyStart) {
			continue
		}
		nameIdx = candidateName
		paramStart = idx
		paramEnd = end
		break
	}
	return nameIdx, paramStart, paramEnd, nameIdx >= 0
}

func javaLooksLikeMethodHeader(tokens []parserToken, start int, nameIdx int, paramEnd int, bodyStart int) bool {
	hasHeaderPrefix := false
	for idx := start; idx < nameIdx; idx++ {
		switch tokens[idx].text {
		case ".", "->":
			return false
		case "new":
			return false
		case "@":
			hasHeaderPrefix = true
		default:
			if isParserIdentifier(tokens[idx].text) || tokens[idx].text == "<" || tokens[idx].text == ">" || tokens[idx].text == "[" || tokens[idx].text == "]" {
				hasHeaderPrefix = true
			}
		}
	}
	if !hasHeaderPrefix {
		return false
	}
	for idx := paramEnd + 1; idx < bodyStart; idx++ {
		switch tokens[idx].text {
		case "=", "(":
			return false
		case "-":
			if idx+1 < bodyStart && tokens[idx+1].text == ">" {
				return false
			}
		}
	}
	return true
}
