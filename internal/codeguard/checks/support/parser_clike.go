package support

import "strings"

type parserToken struct {
	text  string
	start int
	end   int
	line  int
}

func tokenizeCLikeSource(source string, skipRawStrings bool) []parserToken {
	tokens := make([]parserToken, 0)
	line := 1
	for idx := 0; idx < len(source); {
		ch := source[idx]
		switch ch {
		case ' ', '\t', '\r':
			idx++
		case '\n':
			line++
			idx++
		case '/':
			switch {
			case idx+1 < len(source) && source[idx+1] == '/':
				idx += 2
				for idx < len(source) && source[idx] != '\n' {
					idx++
				}
			case idx+1 < len(source) && source[idx+1] == '*':
				idx += 2
				for idx < len(source) {
					if source[idx] == '\n' {
						line++
					}
					if idx+1 < len(source) && source[idx] == '*' && source[idx+1] == '/' {
						idx += 2
						break
					}
					idx++
				}
			default:
				tokens = append(tokens, parserToken{text: string(ch), start: idx, end: idx + 1, line: line})
				idx++
			}
		case '"':
			nextIdx, nextLine := skipQuotedLiteral(source, idx, line, '"')
			idx, line = nextIdx, nextLine
		case '\'':
			nextIdx, nextLine := skipQuotedLiteral(source, idx, line, '\'')
			idx, line = nextIdx, nextLine
		default:
			if skipRawStrings {
				if nextIdx, nextLine, ok := skipRustStringLiteral(source, idx, line); ok {
					idx, line = nextIdx, nextLine
					continue
				}
			}
			if isParserIdentStart(ch) {
				start := idx
				idx++
				for idx < len(source) && isParserIdentPart(source[idx]) {
					idx++
				}
				tokens = append(tokens, parserToken{text: source[start:idx], start: start, end: idx, line: line})
				continue
			}
			tokens = append(tokens, parserToken{text: string(ch), start: idx, end: idx + 1, line: line})
			idx++
		}
	}
	return tokens
}

func skipQuotedLiteral(source string, start int, line int, quote byte) (int, int) {
	idx := start + 1
	currentLine := line
	for idx < len(source) {
		if source[idx] == '\n' {
			currentLine++
		}
		if source[idx] == '\\' && idx+1 < len(source) {
			idx += 2
			continue
		}
		if source[idx] == quote {
			return idx + 1, currentLine
		}
		idx++
	}
	return len(source), currentLine
}

func skipRustStringLiteral(source string, start int, line int) (int, int, bool) {
	if start >= len(source) {
		return start, line, false
	}
	switch source[start] {
	case 'b':
		if start+1 < len(source) && source[start+1] == '"' {
			nextIdx, nextLine := skipQuotedLiteral(source, start+1, line, '"')
			return nextIdx, nextLine, true
		}
		if start+1 < len(source) && source[start+1] == '\'' {
			nextIdx, nextLine := skipQuotedLiteral(source, start+1, line, '\'')
			return nextIdx, nextLine, true
		}
		if start+1 < len(source) && source[start+1] == 'r' {
			return skipRustRawStringLiteral(source, start, line)
		}
	case 'r':
		return skipRustRawStringLiteral(source, start, line)
	}
	return start, line, false
}

func skipRustRawStringLiteral(source string, start int, line int) (int, int, bool) {
	prefixLen := 1
	if source[start] == 'b' {
		prefixLen = 2
		if start+1 >= len(source) || source[start+1] != 'r' {
			return start, line, false
		}
	}
	idx := start + prefixLen
	hashes := 0
	for idx < len(source) && source[idx] == '#' {
		hashes++
		idx++
	}
	if idx >= len(source) || source[idx] != '"' {
		return start, line, false
	}
	idx++
	currentLine := line
	terminator := `"` + strings.Repeat("#", hashes)
	for idx < len(source) {
		if source[idx] == '\n' {
			currentLine++
		}
		if strings.HasPrefix(source[idx:], terminator) {
			return idx + len(terminator), currentLine, true
		}
		idx++
	}
	return len(source), currentLine, true
}

func isParserIdentStart(ch byte) bool {
	return ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z'
}

func isParserIdentPart(ch byte) bool {
	return isParserIdentStart(ch) || ch >= '0' && ch <= '9'
}

func findMatchingToken(tokens []parserToken, start int, open string, close string) int {
	depth := 0
	for idx := start; idx < len(tokens); idx++ {
		switch tokens[idx].text {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return idx
			}
		}
	}
	return -1
}
