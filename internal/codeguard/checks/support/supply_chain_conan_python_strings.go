package support

import (
	"strconv"
	"strings"
)

func scanPythonStringToken(source string, position pythonScanPosition, prefix string) (pythonToken, pythonScanPosition) {
	tokenLine, tokenColumn := position.line, position.column-len(prefix)
	value, next, ok := scanPythonString(source, position)
	kind := byte('s')
	if !ok || strings.Contains(strings.ToLower(prefix), "f") {
		kind = 'x'
		value = prefix + value
	}
	return pythonToken{kind: kind, value: value, line: tokenLine, column: tokenColumn}, next
}

func scanPythonString(source string, position pythonScanPosition) (string, pythonScanPosition, bool) {
	quote := source[position.offset]
	triple := position.offset+2 < len(source) && source[position.offset+1] == quote && source[position.offset+2] == quote
	delimiterLength := 1
	if triple {
		delimiterLength = 3
	}
	start := position.offset
	position.offset += delimiterLength
	position.column += delimiterLength
	for position.offset < len(source) {
		if source[position.offset] == '\\' {
			position = skipPythonEscape(source, position)
			continue
		}
		if source[position.offset] == '\n' {
			if !triple {
				return source[start:position.offset], position, false
			}
			position.offset++
			position.line++
			position.column = 1
			continue
		}
		if pythonStringClosesAt(source, position.offset, quote, triple) {
			return finishPythonString(source, start, position, delimiterLength, triple)
		}
		position.offset++
		position.column++
	}
	return source[start:], position, false
}

func skipPythonEscape(source string, position pythonScanPosition) pythonScanPosition {
	advance := min(2, len(source)-position.offset)
	if advance == 2 && source[position.offset+1] == '\n' {
		position.line++
		position.column = 1
	} else {
		position.column += advance
	}
	position.offset += advance
	return position
}

func pythonStringClosesAt(source string, offset int, quote byte, triple bool) bool {
	if source[offset] != quote {
		return false
	}
	return !triple || (offset+2 < len(source) && source[offset+1] == quote && source[offset+2] == quote)
}

func finishPythonString(source string, start int, position pythonScanPosition, delimiterLength int, triple bool) (string, pythonScanPosition, bool) {
	end := position.offset + delimiterLength
	raw := source[start:end]
	value, err := strconv.Unquote(raw)
	if triple {
		value = source[start+3 : position.offset]
		err = nil
	}
	position.offset = end
	position.column += delimiterLength
	return value, position, err == nil
}

func isPythonStringPrefix(value string) bool {
	if len(value) > 2 {
		return false
	}
	for _, char := range strings.ToLower(value) {
		if !strings.ContainsRune("rubf", char) {
			return false
		}
	}
	return value != ""
}
