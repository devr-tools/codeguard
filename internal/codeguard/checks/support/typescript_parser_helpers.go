package support

import "strings"

func (parser *scriptCallArgumentParser) decrementBraceDepth() {
	if parser.braceDepth > 0 {
		parser.braceDepth--
	}
}

func (parser *scriptCallArgumentParser) decrementBracketDepth() {
	if parser.bracketDepth > 0 {
		parser.bracketDepth--
	}
}

func (parser *scriptCallArgumentParser) splitArgument(idx int) {
	if parser.parenDepth != 1 || parser.braceDepth != 0 || parser.bracketDepth != 0 {
		return
	}
	parser.appendArgument(idx)
	parser.start = idx + 1
}

func (parser *scriptCallArgumentParser) appendArgument(end int) {
	arg := strings.TrimSpace(parser.source[parser.start:end])
	if arg != "" {
		parser.args = append(parser.args, arg)
	}
}

func (parser *scriptCallArgumentParser) matchesAt(idx int, token string) bool {
	return idx+len(token) <= len(parser.source) && parser.source[idx:idx+len(token)] == token
}
