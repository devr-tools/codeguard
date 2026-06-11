package support

type scriptParserState string

const (
	scriptParserCode         scriptParserState = "code"
	scriptParserLineComment  scriptParserState = "line-comment"
	scriptParserBlockComment scriptParserState = "block-comment"
	scriptParserSingleQuote  scriptParserState = "'"
	scriptParserDoubleQuote  scriptParserState = `"`
	scriptParserTemplate     scriptParserState = "template"
)

type scriptCallArgumentParser struct {
	source       string
	args         []string
	start        int
	parenDepth   int
	braceDepth   int
	bracketDepth int
	state        scriptParserState
}

func parseScriptCallArguments(source string, openParen int) []string {
	if openParen < 0 || openParen >= len(source) || source[openParen] != '(' {
		return nil
	}

	parser := scriptCallArgumentParser{
		source:     source,
		args:       make([]string, 0, 4),
		start:      openParen + 1,
		parenDepth: 1,
		state:      scriptParserCode,
	}
	for idx := openParen + 1; idx < len(source); idx++ {
		if parser.consumeNonCode(&idx) || parser.beginComment(&idx) {
			continue
		}
		if parser.handleCodeByte(idx) {
			return parser.args
		}
	}

	return parser.args
}

func (parser *scriptCallArgumentParser) consumeNonCode(idx *int) bool {
	ch := parser.source[*idx]
	switch parser.state {
	case scriptParserLineComment:
		if ch == '\n' {
			parser.state = scriptParserCode
		}
		return true
	case scriptParserBlockComment:
		if parser.matchesAt(*idx, "*/") {
			parser.state = scriptParserCode
			*idx = *idx + 1
		}
		return true
	case scriptParserSingleQuote:
		return parser.consumeQuotedState(idx, ch, '\'')
	case scriptParserDoubleQuote:
		return parser.consumeQuotedState(idx, ch, '"')
	case scriptParserTemplate:
		return parser.consumeQuotedState(idx, ch, '`')
	default:
		return false
	}
}

func (parser *scriptCallArgumentParser) consumeQuotedState(idx *int, ch byte, quote byte) bool {
	if ch == '\\' && *idx+1 < len(parser.source) {
		*idx = *idx + 1
		return true
	}
	if ch == quote {
		parser.state = scriptParserCode
	}
	return true
}

func (parser *scriptCallArgumentParser) beginComment(idx *int) bool {
	switch {
	case parser.matchesAt(*idx, "//"):
		parser.state = scriptParserLineComment
		*idx = *idx + 1
		return true
	case parser.matchesAt(*idx, "/*"):
		parser.state = scriptParserBlockComment
		*idx = *idx + 1
		return true
	default:
		return false
	}
}

func (parser *scriptCallArgumentParser) handleCodeByte(idx int) bool {
	ch := parser.source[idx]
	if parser.enterLiteralState(ch) || parser.handleOpeningDelimiter(ch) {
		return false
	}
	return parser.handleClosingDelimiter(ch, idx)
}

func (parser *scriptCallArgumentParser) closeParen(idx int) bool {
	parser.parenDepth--
	if parser.parenDepth != 0 {
		return false
	}
	parser.appendArgument(idx)
	return true
}

func (parser *scriptCallArgumentParser) enterLiteralState(ch byte) bool {
	switch ch {
	case '\'':
		parser.state = scriptParserSingleQuote
	case '"':
		parser.state = scriptParserDoubleQuote
	case '`':
		parser.state = scriptParserTemplate
	default:
		return false
	}
	return true
}

func (parser *scriptCallArgumentParser) handleOpeningDelimiter(ch byte) bool {
	switch ch {
	case '(':
		parser.parenDepth++
	case '{':
		parser.braceDepth++
	case '[':
		parser.bracketDepth++
	default:
		return false
	}
	return true
}

func (parser *scriptCallArgumentParser) handleClosingDelimiter(ch byte, idx int) bool {
	switch ch {
	case ')':
		return parser.closeParen(idx)
	case '}':
		parser.decrementBraceDepth()
	case ']':
		parser.decrementBracketDepth()
	case ',':
		parser.splitArgument(idx)
	}
	return false
}
