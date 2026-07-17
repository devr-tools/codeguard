package design

import "strings"

func normalizeJSONC(data []byte) ([]byte, bool) {
	withoutComments, ok := stripJSONCComments(string(data))
	if !ok {
		return nil, false
	}
	return []byte(stripJSONCTrailingCommas(withoutComments)), true
}

func stripJSONCComments(source string) (string, bool) {
	var b strings.Builder
	b.Grow(len(source))
	inString := false
	escaped := false
	for idx := 0; idx < len(source); idx++ {
		ch := source[idx]
		if inString {
			escaped = appendJSONStringByte(&b, ch, escaped, &inString)
			continue
		}
		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			continue
		}
		if next, handled := stripJSONCLineComment(source, idx); handled {
			idx = next
			continue
		}
		next, ok, handled := stripJSONCBlockComment(&b, source, idx)
		if handled {
			if !ok {
				return "", false
			}
			idx = next
			continue
		}
		b.WriteByte(ch)
	}
	return b.String(), !inString
}

func appendJSONStringByte(b *strings.Builder, ch byte, escaped bool, inString *bool) bool {
	b.WriteByte(ch)
	if escaped {
		return false
	}
	switch ch {
	case '\\':
		return true
	case '"':
		*inString = false
	}
	return false
}

func stripJSONCLineComment(source string, idx int) (int, bool) {
	if source[idx] != '/' || idx+1 >= len(source) || source[idx+1] != '/' {
		return idx, false
	}
	for idx+1 < len(source) && source[idx+1] != '\n' {
		idx++
	}
	return idx, true
}

func stripJSONCBlockComment(b *strings.Builder, source string, idx int) (int, bool, bool) {
	if source[idx] != '/' || idx+1 >= len(source) || source[idx+1] != '*' {
		return idx, true, false
	}
	idx += 2
	for idx < len(source) {
		if idx+1 < len(source) && source[idx] == '*' && source[idx+1] == '/' {
			return idx + 1, true, true
		}
		if source[idx] == '\n' {
			b.WriteByte('\n')
		}
		idx++
	}
	return idx, false, true
}

func stripJSONCTrailingCommas(source string) string {
	var b strings.Builder
	b.Grow(len(source))
	inString := false
	escaped := false
	for idx := 0; idx < len(source); idx++ {
		ch := source[idx]
		if inString {
			escaped = appendJSONStringByte(&b, ch, escaped, &inString)
			continue
		}
		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			continue
		}
		if ch == ',' {
			next := idx + 1
			for next < len(source) && isJSONWhitespace(source[next]) {
				next++
			}
			if next < len(source) && (source[next] == '}' || source[next] == ']') {
				continue
			}
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func isJSONWhitespace(ch byte) bool {
	switch ch {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}
