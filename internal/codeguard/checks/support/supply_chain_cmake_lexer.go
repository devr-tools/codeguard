package support

import (
	"strings"
	"unicode"
)

func skipCMakeTrivia(source string, position cmakeScanPosition) cmakeScanPosition {
	for position.offset < len(source) {
		if unicode.IsSpace(rune(source[position.offset])) {
			position = skipCMakeWhitespace(source, position)
			continue
		}
		if source[position.offset] == '#' {
			position = skipCMakeComment(source, position)
			continue
		}
		break
	}
	return position
}

func skipCMakeWhitespace(source string, position cmakeScanPosition) cmakeScanPosition {
	for position.offset < len(source) && unicode.IsSpace(rune(source[position.offset])) {
		if source[position.offset] == '\n' {
			position.line++
		}
		position.offset++
	}
	return position
}

func scanCMakeQuoted(source string, position cmakeScanPosition) (string, cmakeScanPosition, bool) {
	var value strings.Builder
	for position.offset++; position.offset < len(source); position.offset++ {
		if source[position.offset] == '\n' {
			position.line++
		}
		if source[position.offset] == '\\' && position.offset+1 < len(source) {
			position.offset++
			if source[position.offset] == '\n' {
				position.line++
			}
			value.WriteByte(source[position.offset])
			continue
		}
		if source[position.offset] == '"' {
			position.offset++
			return value.String(), position, true
		}
		value.WriteByte(source[position.offset])
	}
	return value.String(), position, false
}

func scanCMakeBracketArgument(source string, position cmakeScanPosition) (string, cmakeScanPosition, bool) {
	start := position.offset
	end := start + 1
	for end < len(source) && source[end] == '=' {
		end++
	}
	if end >= len(source) || source[end] != '[' {
		return "", position, false
	}
	closing := "]" + strings.Repeat("=", end-start-1) + "]"
	contentStart := end + 1
	closeAt := strings.Index(source[contentStart:], closing)
	if closeAt < 0 {
		position.line += strings.Count(source[start:], "\n")
		position.offset = len(source)
		return "", position, false
	}
	value := source[contentStart : contentStart+closeAt]
	position.offset = contentStart + closeAt + len(closing)
	position.line += strings.Count(source[start:position.offset], "\n")
	return value, position, true
}

func skipCMakeComment(source string, position cmakeScanPosition) cmakeScanPosition {
	bracketPosition := position
	bracketPosition.offset++
	if _, next, ok := scanCMakeBracketArgument(source, bracketPosition); ok {
		return next
	}
	for position.offset < len(source) && source[position.offset] != '\n' {
		position.offset++
	}
	return position
}

func isCMakeIdentifierStart(char byte) bool {
	return unicode.IsLetter(rune(char)) || char == '_'
}

func isCMakeIdentifierPart(char byte) bool {
	return isCMakeIdentifierStart(char) || unicode.IsDigit(rune(char))
}
