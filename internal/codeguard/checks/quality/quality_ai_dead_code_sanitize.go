package quality

// Lexer states for sanitizeScriptSource.
const (
	scriptCode = iota
	scriptLineComment
	scriptBlockComment
	scriptSingleQuote
	scriptDoubleQuote
	scriptTemplateQuote
)

// sanitizeScriptSource blanks out comment and string contents while keeping
// newlines so that brace tracking and line numbers stay accurate.
func sanitizeScriptSource(source string) string {
	out := []rune(source)
	state := scriptCode
	for i := 0; i < len(out); i++ {
		switch state {
		case scriptCode:
			state = scanScriptCodeRune(out, i)
		case scriptLineComment:
			state = blankScriptLineCommentRune(out, i)
		case scriptBlockComment:
			state, i = blankScriptBlockCommentRune(out, i)
		default:
			state, i = blankScriptStringRune(out, i, state)
		}
	}
	return string(out)
}

func scanScriptCodeRune(out []rune, i int) int {
	switch ch, next := out[i], scriptRuneAt(out, i+1); {
	case ch == '/' && next == '/':
		out[i] = ' '
		return scriptLineComment
	case ch == '/' && next == '*':
		out[i] = ' '
		return scriptBlockComment
	case ch == '\'':
		return scriptSingleQuote
	case ch == '"':
		return scriptDoubleQuote
	case ch == '`':
		return scriptTemplateQuote
	default:
		return scriptCode
	}
}

func blankScriptLineCommentRune(out []rune, i int) int {
	if out[i] == '\n' {
		return scriptCode
	}
	out[i] = ' '
	return scriptLineComment
}

func blankScriptBlockCommentRune(out []rune, i int) (int, int) {
	if out[i] == '*' && scriptRuneAt(out, i+1) == '/' {
		out[i] = ' '
		out[i+1] = ' '
		return scriptCode, i + 1
	}
	if out[i] != '\n' {
		out[i] = ' '
	}
	return scriptBlockComment, i
}

// blankScriptStringRune blanks one rune inside a string literal, consuming
// escape sequences so escaped closers do not end the literal early.
func blankScriptStringRune(out []rune, i int, state int) (int, int) {
	closer := map[int]rune{scriptSingleQuote: '\'', scriptDoubleQuote: '"', scriptTemplateQuote: '`'}[state]
	switch ch := out[i]; {
	case ch == '\\':
		out[i] = ' '
		if i+1 < len(out) && out[i+1] != '\n' {
			out[i+1] = ' '
			i++
		}
	case ch == closer:
		return scriptCode, i
	case ch != '\n':
		out[i] = ' '
	}
	return state, i
}

func scriptRuneAt(out []rune, i int) rune {
	if i < len(out) {
		return out[i]
	}
	return 0
}
