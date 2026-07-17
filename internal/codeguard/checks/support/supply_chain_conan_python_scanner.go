package support

import (
	"unicode"
)

type pythonToken struct {
	kind   byte
	value  string
	line   int
	column int
}

type pythonScanPosition struct {
	offset int
	line   int
	column int
}

func scanPythonTokens(source string) []pythonToken {
	tokens := make([]pythonToken, 0)
	position := pythonScanPosition{line: 1, column: 1}
	for position.offset < len(source) {
		position = skipPythonTrivia(source, position)
		if position.offset >= len(source) {
			break
		}
		token, next := scanPythonTokenAt(source, position)
		tokens = append(tokens, token)
		position = next
	}
	return tokens
}

func skipPythonTrivia(source string, position pythonScanPosition) pythonScanPosition {
	for position.offset < len(source) {
		char := source[position.offset]
		if char == ' ' || char == '\t' || char == '\r' {
			position.offset++
			position.column++
			continue
		}
		if char != '#' {
			break
		}
		for position.offset < len(source) && source[position.offset] != '\n' {
			position.offset++
			position.column++
		}
	}
	return position
}

func scanPythonTokenAt(source string, position pythonScanPosition) (pythonToken, pythonScanPosition) {
	char := source[position.offset]
	if char == '\n' {
		token := pythonToken{kind: 'n', value: "\n", line: position.line, column: position.column}
		position.offset++
		position.line++
		position.column = 1
		return token, position
	}
	if isPythonIdentifierStart(char) {
		return scanPythonIdentifierToken(source, position)
	}
	if char == '\'' || char == '"' {
		return scanPythonStringToken(source, position, "")
	}
	token := pythonToken{kind: 'p', value: string(char), line: position.line, column: position.column}
	position.offset++
	position.column++
	return token, position
}

func scanPythonIdentifierToken(source string, position pythonScanPosition) (pythonToken, pythonScanPosition) {
	start, tokenLine, tokenColumn := position.offset, position.line, position.column
	for position.offset < len(source) && isPythonIdentifierPart(source[position.offset]) {
		position.offset++
		position.column++
	}
	identifier := source[start:position.offset]
	if position.offset < len(source) && (source[position.offset] == '\'' || source[position.offset] == '"') && isPythonStringPrefix(identifier) {
		return scanPythonStringToken(source, position, identifier)
	}
	return pythonToken{kind: 'i', value: identifier, line: tokenLine, column: tokenColumn}, position
}

func isPythonIdentifierStart(char byte) bool {
	return char == '_' || unicode.IsLetter(rune(char))
}

func isPythonIdentifierPart(char byte) bool {
	return isPythonIdentifierStart(char) || unicode.IsDigit(rune(char))
}
