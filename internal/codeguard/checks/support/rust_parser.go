package support

func ParseRustFunctions(source string) []ParsedFunction {
	tokens := tokenizeCLikeSource(source, true)
	functions := make([]ParsedFunction, 0)
	for idx := 0; idx < len(tokens); idx++ {
		if tokens[idx].text != "fn" {
			continue
		}
		if idx+2 >= len(tokens) || !isParserIdentifier(tokens[idx+1].text) {
			continue
		}
		nameTok := tokens[idx+1]
		paramStart := idx + 2
		if paramStart < len(tokens) && tokens[paramStart].text == "<" {
			paramEnd := findMatchingToken(tokens, paramStart, "<", ">")
			if paramEnd < 0 {
				continue
			}
			paramStart = paramEnd + 1
		}
		if paramStart >= len(tokens) || tokens[paramStart].text != "(" {
			continue
		}
		paramEnd := findMatchingToken(tokens, paramStart, "(", ")")
		if paramEnd < 0 {
			continue
		}
		bodyStart := -1
		for j := paramEnd + 1; j < len(tokens); j++ {
			switch tokens[j].text {
			case "{":
				bodyStart = j
				j = len(tokens)
			case ";":
				j = len(tokens)
			}
		}
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
