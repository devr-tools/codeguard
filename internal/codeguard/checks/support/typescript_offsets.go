package support

import "strings"

func LineNumberForOffset(source string, offset int) int {
	if offset <= 0 {
		return 1
	}
	if offset > len(source) {
		offset = len(source)
	}
	return 1 + strings.Count(source[:offset], "\n")
}

func PreviousSignificantByte(source string, idx int) byte {
	for i := idx - 1; i >= 0; i-- {
		switch source[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return source[i]
		}
	}
	return 0
}

func NextSignificantByte(source string, idx int) byte {
	for i := idx; i < len(source); i++ {
		switch source[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return source[i]
		}
	}
	return 0
}

func IsTypeScriptAssertionTarget(ch byte) bool {
	switch {
	case ch == ')' || ch == ']' || ch == '}' || ch == '$' || ch == '_':
		return true
	case ch >= '0' && ch <= '9':
		return true
	case ch >= 'A' && ch <= 'Z':
		return true
	case ch >= 'a' && ch <= 'z':
		return true
	default:
		return false
	}
}
