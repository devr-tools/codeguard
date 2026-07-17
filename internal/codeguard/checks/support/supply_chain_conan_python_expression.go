package support

func evaluateConanLiteralExpression(tokens []pythonToken, constants map[string][]conanLiteral) ([]conanLiteral, bool) {
	for len(tokens) > 0 && tokens[0].kind == 'n' {
		tokens = tokens[1:]
	}
	for len(tokens) > 0 && tokens[len(tokens)-1].kind == 'n' {
		tokens = tokens[:len(tokens)-1]
	}
	if len(tokens) == 0 {
		return nil, false
	}
	parser := conanExpressionParser{tokens: tokens, constants: constants}
	values, ok := parser.parseSequence(0)
	for parser.pos < len(tokens) && tokens[parser.pos].kind == 'n' {
		parser.pos++
	}
	return values, ok && parser.pos == len(tokens)
}

type conanExpressionParser struct {
	tokens    []pythonToken
	constants map[string][]conanLiteral
	pos       int
}

func (parser *conanExpressionParser) parseSequence(closing byte) ([]conanLiteral, bool) {
	values := make([]conanLiteral, 0)
	for parser.pos < len(parser.tokens) {
		parser.skipNewlines()
		if closing != 0 && parser.pos < len(parser.tokens) && parser.tokens[parser.pos].value == string(closing) {
			parser.pos++
			return values, true
		}
		atom, ok := parser.parseAtom()
		if !ok {
			return nil, false
		}
		values = append(values, atom...)
		parser.skipNewlines()
		if parser.pos < len(parser.tokens) && parser.tokens[parser.pos].value == "+" {
			// Python's + is overloaded for strings and sequences. Without
			// executing or type-checking the recipe, treating it as either can
			// invent a dependency, so leave it explicitly unresolved.
			return nil, false
		}
		if parser.pos < len(parser.tokens) && parser.tokens[parser.pos].value == "," {
			parser.pos++
			continue
		}
		if closing == 0 {
			return values, true
		}
		if parser.pos < len(parser.tokens) && parser.tokens[parser.pos].value == string(closing) {
			parser.pos++
			return values, true
		}
		return nil, false
	}
	return values, closing == 0
}

func (parser *conanExpressionParser) parseAtom() ([]conanLiteral, bool) {
	parser.skipNewlines()
	if parser.pos >= len(parser.tokens) {
		return nil, false
	}
	token := parser.tokens[parser.pos]
	parser.pos++
	switch {
	case token.kind == 's':
		return []conanLiteral{{value: token.value, line: token.line}}, true
	case token.kind == 'i':
		values, ok := parser.constants[token.value]
		if !ok {
			return nil, false
		}
		return append([]conanLiteral(nil), values...), true
	case token.value == "(":
		return parser.parseSequence(')')
	case token.value == "[":
		return parser.parseSequence(']')
	default:
		return nil, false
	}
}

func (parser *conanExpressionParser) skipNewlines() {
	for parser.pos < len(parser.tokens) && parser.tokens[parser.pos].kind == 'n' {
		parser.pos++
	}
}

func pythonExpressionEnd(tokens []pythonToken, start int) int {
	depth := 0
	for idx := start; idx < len(tokens); idx++ {
		switch tokens[idx].value {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			if depth > 0 {
				depth--
			}
		}
		if tokens[idx].kind == 'n' && depth == 0 {
			return idx
		}
	}
	return len(tokens)
}

func matchingPythonDelimiter(tokens []pythonToken, open int) int {
	depth := 0
	for idx := open; idx < len(tokens); idx++ {
		switch tokens[idx].value {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return idx
			}
		}
	}
	return -1
}

func firstPythonCallArgumentEnd(tokens []pythonToken) int {
	depth := 0
	for idx, token := range tokens {
		switch token.value {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case ",":
			if depth == 0 {
				return idx
			}
		}
	}
	return len(tokens)
}
