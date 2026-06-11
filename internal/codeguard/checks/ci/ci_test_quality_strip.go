package ci

import "strings"

type commentStripState struct {
	inBlockComment bool
	inSingleQuote  bool
	inDoubleQuote  bool
	inBacktick     bool
}

func stripCommentContent(line string, ext string, inBlockComment bool) (string, bool) {
	if ext == ".py" || ext == ".rb" {
		return stripLineComment(line, "#"), false
	}

	var out strings.Builder
	state := commentStripState{inBlockComment: inBlockComment}
	for i := 0; i < len(line); {
		next, done := state.advance(line, i, &out)
		if done {
			return out.String(), state.inBlockComment
		}
		i = next
	}

	return out.String(), state.inBlockComment
}

func (state *commentStripState) advance(line string, i int, out *strings.Builder) (int, bool) {
	if state.inBlockComment {
		end := strings.Index(line[i:], "*/")
		if end == -1 {
			return len(line), true
		}
		state.inBlockComment = false
		return i + end + 2, false
	}
	if quote, ok := state.activeQuote(); ok {
		return state.advanceQuoted(line, i, quote, out), false
	}
	if strings.HasPrefix(line[i:], "//") {
		return len(line), true
	}
	if strings.HasPrefix(line[i:], "/*") {
		state.inBlockComment = true
		return i + 2, false
	}
	return state.advanceCode(line, i, out), false
}

func (state *commentStripState) activeQuote() (byte, bool) {
	switch {
	case state.inSingleQuote:
		return '\'', true
	case state.inDoubleQuote:
		return '"', true
	case state.inBacktick:
		return '`', true
	default:
		return 0, false
	}
}

func (state *commentStripState) advanceQuoted(line string, i int, quote byte, out *strings.Builder) int {
	out.WriteByte(line[i])
	if line[i] == '\\' && quote != '`' && i+1 < len(line) {
		out.WriteByte(line[i+1])
		return i + 2
	}
	if line[i] == quote {
		state.clearQuote(quote)
	}
	return i + 1
}

func (state *commentStripState) advanceCode(line string, i int, out *strings.Builder) int {
	switch line[i] {
	case '\'':
		state.inSingleQuote = true
	case '"':
		state.inDoubleQuote = true
	case '`':
		state.inBacktick = true
	}
	out.WriteByte(line[i])
	return i + 1
}

func (state *commentStripState) clearQuote(quote byte) {
	switch quote {
	case '\'':
		state.inSingleQuote = false
	case '"':
		state.inDoubleQuote = false
	case '`':
		state.inBacktick = false
	}
}
