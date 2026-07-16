package compdb

import (
	"errors"
	"strings"
)

type commandLexer struct {
	arguments []string
	current   strings.Builder
	quote     rune
	escaped   bool
	started   bool
}

// splitCommandLine tokenizes the command form for metadata extraction only.
// Its result is never used to select or execute the database-provided program.
func splitCommandLine(command string) ([]string, error) {
	lexer := commandLexer{}
	for _, character := range command {
		lexer.consume(character)
	}
	if lexer.escaped || lexer.quote != 0 {
		return nil, errors.New("unterminated quote or escape in compilation command")
	}
	lexer.flush()
	return lexer.arguments, nil
}

func (lexer *commandLexer) consume(character rune) {
	if lexer.consumeEscaped(character) || lexer.consumeQuoted(character) {
		return
	}
	switch character {
	case '\\':
		lexer.escaped, lexer.started = true, true
	case '\'', '"':
		lexer.quote, lexer.started = character, true
	case ' ', '\t', '\r', '\n':
		lexer.flush()
	default:
		lexer.current.WriteRune(character)
		lexer.started = true
	}
}

func (lexer *commandLexer) consumeEscaped(character rune) bool {
	if !lexer.escaped {
		return false
	}
	lexer.current.WriteRune(character)
	lexer.escaped, lexer.started = false, true
	return true
}

func (lexer *commandLexer) consumeQuoted(character rune) bool {
	if lexer.quote == 0 {
		return false
	}
	switch {
	case character == lexer.quote:
		lexer.quote = 0
	case character == '\\' && lexer.quote != '\'':
		lexer.escaped = true
	default:
		lexer.current.WriteRune(character)
	}
	lexer.started = true
	return true
}

func (lexer *commandLexer) flush() {
	if !lexer.started {
		return
	}
	lexer.arguments = append(lexer.arguments, lexer.current.String())
	lexer.current.Reset()
	lexer.started = false
}
