package support

func ParseRustFunctions(source string) []ParsedFunction {
	tokens := tokenizeCLikeSource(source, true)
	functions := make([]ParsedFunction, 0)
	for idx := 0; idx < len(tokens); idx++ {
		if tokens[idx].text != "fn" {
			continue
		}
		nameTok, paramStart, ok := rustFunctionSignature(tokens, idx)
		if !ok {
			continue
		}
		paramEnd := findMatchingToken(tokens, paramStart, "(", ")")
		if paramEnd < 0 {
			continue
		}
		bodyStart := rustFunctionBodyStart(tokens, paramEnd+1)
		if bodyStart < 0 {
			continue
		}
		bodyEnd := findMatchingToken(tokens, bodyStart, "{", "}")
		if bodyEnd < 0 {
			continue
		}
		functions = append(functions, ParsedFunction{
			Name:       nameTok.text,
			StartLine:  tokens[idx].line,
			EndLine:    tokens[bodyEnd].line,
			Parameters: source[tokens[paramStart].end:tokens[paramEnd].start],
			Body:       source[tokens[bodyStart].end:tokens[bodyEnd].start],
		})
	}
	return functions
}

func rustFunctionSignature(tokens []parserToken, idx int) (parserToken, int, bool) {
	if idx+2 >= len(tokens) || !isParserIdentifier(tokens[idx+1].text) {
		return parserToken{}, 0, false
	}
	paramStart := idx + 2
	if paramStart < len(tokens) && tokens[paramStart].text == "<" {
		paramEnd := findMatchingToken(tokens, paramStart, "<", ">")
		if paramEnd < 0 {
			return parserToken{}, 0, false
		}
		paramStart = paramEnd + 1
	}
	if paramStart >= len(tokens) || tokens[paramStart].text != "(" {
		return parserToken{}, 0, false
	}
	return tokens[idx+1], paramStart, true
}

func rustFunctionBodyStart(tokens []parserToken, start int) int {
	for j := start; j < len(tokens); j++ {
		switch tokens[j].text {
		case "{":
			return j
		case ";":
			return -1
		}
	}
	return -1
}

func isParserIdentifier(token string) bool {
	if token == "" || !isParserIdentStart(token[0]) {
		return false
	}
	for idx := 1; idx < len(token); idx++ {
		if !isParserIdentPart(token[idx]) {
			return false
		}
	}
	return true
}
