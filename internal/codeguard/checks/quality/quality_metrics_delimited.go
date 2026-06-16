package quality

import "strings"

type delimiterState struct {
	depthParen   int
	depthBracket int
	depthBrace   int
	depthAngle   int
	inString     byte
}

func (state *delimiterState) atTopLevel() bool {
	return state.depthParen == 0 && state.depthBracket == 0 && state.depthBrace == 0 && state.depthAngle == 0
}

func (state *delimiterState) advance(ch byte) {
	switch ch {
	case '"', '\'':
		state.inString = ch
	case '(':
		state.depthParen++
	case ')':
		state.depthParen = max(0, state.depthParen-1)
	case '[':
		state.depthBracket++
	case ']':
		state.depthBracket = max(0, state.depthBracket-1)
	case '{':
		state.depthBrace++
	case '}':
		state.depthBrace = max(0, state.depthBrace-1)
	case '<':
		state.depthAngle++
	case '>':
		state.depthAngle = max(0, state.depthAngle-1)
	}
}

func shouldSkipDelimitedStringByte(signature string, idx int, state *delimiterState) bool {
	ch := signature[idx]
	if ch == '\\' && idx+1 < len(signature) {
		return true
	}
	if ch == state.inString {
		state.inString = 0
	}
	return false
}

func appendDelimitedPart(parts []string, raw string) []string {
	if part := strings.TrimSpace(raw); part != "" {
		return append(parts, part)
	}
	return parts
}
